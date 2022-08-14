package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	DM "github.com/kycklingar/PBooru/DataManager"
	paginate "github.com/kycklingar/PBooru/handlers/paginator"
)

type Sidebar struct {
	TotalPosts int
	Tags       []DM.Tag
	Form       url.Values
	Query      string
	Or         string
	Filter     string
	Unless     string
	Alts       bool
	AltGroup   int

	Mimes map[string][]*DM.Mime
}

const tagLimit = 100

func TagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var (
			err    error
			tagStr = r.FormValue("tag")
			uri    = uriSplitter(r)
			p      struct {
				Query         string
				Tags          []DM.Tag
				Tag           *DM.Tag
				To            *DM.Tag
				From          []DM.Tag
				Parents       []DM.Tag
				GrandParents  []DM.Tag
				Children      []DM.Tag
				GrandChildren []DM.Tag
				Paginator     paginate.Paginator
				CurrentPage   int
			}
		)

		p.CurrentPage, err = uri.getIntAtIndex(1)
		if err != nil {
			p.CurrentPage = 1
		}

		result, err := DM.SearchTags(tagStr, tagLimit, (p.CurrentPage-1)*tagLimit)
		if internalError(w, err) {
			return
		}

		p.Tags = result.Tags
		p.Query = r.Form.Encode()
		if p.Query != "" {
			p.Query = "?" + p.Query
		}

		var currentTag string

		if id, err := uri.getIntAtIndex(2); err == nil {
			currentTag, _ = uri.getAtIndex(2)

			tag, err := DM.TagFromID(id)
			p.To, p.From, err = tag.Aliasing()
			if internalError(w, err) {
				return
			}

			p.Children, p.Parents, p.GrandChildren, p.GrandParents, err = tag.Family()
			if internalError(w, err) {
				return
			}
			p.Tag = &tag
		}

		p.Paginator = paginate.New(
			p.CurrentPage,
			result.Count,
			tagLimit,
			30,
			fmt.Sprintf("/tags/%%d/%s%s", currentTag, strings.ReplaceAll(p.Query, "%", "%%")),
		)

		renderTemplate(w, "tags", p)
		return
	}

	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Tags() {
		http.Error(w, "Insufficient privileges. Want \"Tags\"", http.StatusForbidden)
		return
	}

	ua := DM.UserAction(user)

	from := r.FormValue("from")
	to := r.FormValue("to")

	children := r.FormValue("child")
	parents := r.FormValue("parent")

	switch r.FormValue("action") {
	case "alias":
		ua.Add(DM.AliasTags(from, to))
	case "unalias":
		ua.Add(DM.UnaliasTags(from))
	case "parent":
		ua.Add(DM.ParentTags(children, parents))
	case "unparent":
		ua.Add(DM.UnparentTags(children, parents))
	}

	if internalError(w, ua.Exec()) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func multiTagsHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	err := r.ParseMultipartForm(50000000)
	if badRequest(w, err) {
		return
	}

	var (
		addStr = r.Form.Get("tags-add")
		remStr = r.Form.Get("tags-remove")
		pids   []int
	)

	for _, pidstr := range r.Form["pid"] {
		pid, err := strconv.Atoi(pidstr)
		if badRequest(w, err) {
			return
		}

		pids = append(pids, pid)
	}

	ua := DM.UserAction(user)
	ua.Add(DM.AlterManyPostTags(pids, addStr, remStr, '\n'))
	err = ua.Exec()
	if internalError(w, err) {
		return
	}

	fmt.Fprint(w, "OK")
}
