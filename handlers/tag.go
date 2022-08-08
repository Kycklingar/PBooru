package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
	paginate "github.com/kycklingar/PBooru/handlers/paginator"
)

type Sidebar struct {
	TotalPosts int
	Tags       []*DM.Tag
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
		var p struct {
			Query    string
			Tags     []*DM.Tag
			Tag      *DM.Tag
			To       *DM.Tag
			From     []*DM.Tag
			Parents  []*DM.Tag
			Children []*DM.Tag
			//Paginator Pageination
			Paginator paginate.Paginator
		}

		tagStr := r.FormValue("tag")
		currPage := 1
		var err error
		f := splitURI(r.URL.Path)
		if len(f) >= 2 && f[1] != "" {
			currPage, err = strconv.Atoi(f[1])
			if internalError(w, err) {
				return
			}
		}

		bm := benchmark.Begin()
		var tc DM.TagCollector

		if len(tagStr) > 0 {
			tc.Parse(tagStr, ",")
			tc = tc.SuggestedTags(DM.DB)
			p.Query = "?tag=" + tagStr
		} else {
			err = tc.Get(tagLimit, (currPage-1)*tagLimit)
			if internalError(w, err) {
				return
			}
		}

		for _, t := range tc.Tags {
			t.QTag(DM.DB)
			t.QNamespace(DM.DB).QNamespace(DM.DB)
		}

		p.Tags = tc.Tags

		var ctag string

		if len(f) >= 3 && f[2] != "" {
			ctag = f[2]
			tagID, err := strconv.Atoi(f[2])
			if internalError(w, err) {
				return
			}

			preload := func(tags ...*DM.Tag) {
				for _, tag := range tags {
					tag.QID(DM.DB)
					tag.QTag(DM.DB)
					tag.QNamespace(DM.DB).QNamespace(DM.DB)
				}
			}

			a := DM.NewAlias()
			a.Tag.SetID(tagID)
			preload(a.Tag)

			p.Tag = a.Tag
			p.From = a.QFrom(DM.DB)
			preload(p.From...)

			p.To, err = a.QTo(DM.DB)
			if internalError(w, err) {
				return
			}
			preload(p.To)

			p.Parents = p.Tag.Parents(DM.DB)
			preload(p.Parents...)

			p.Children = p.Tag.Children(DM.DB)
			preload(p.Children...)

			bm.End(performBenchmarks)
		}

		p.Paginator = paginate.New(
			currPage,
			tc.Total(),
			tagLimit,
			30,
			fmt.Sprintf("/tags/%%d/%s%s", ctag, strings.ReplaceAll(p.Query, "%", "%%")),
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
		//case "unparent":
		//	ua.Add(DM.UnparentTags(children, parents))
	}

	err := ua.Exec()
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

type TagHistoryPage struct {
	Base       base
	History    []*DM.TagHistory
	UserInfo   UserInfo
	Pageinator Pageination
	User       *DM.User
}

func ReverseTagHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		u, _ := getUser(w, r)

		if !u.QFlag(DM.DB).Delete() {
			http.Error(w, "Insufficent privileges. Want \"delete\"", http.StatusBadRequest)
			return
		}
		var (
			th  = DM.NewTagHistory()
			err error
		)

		th.ID, err = strconv.Atoi(r.FormValue("taghistory-id"))
		if badRequest(w, err) {
			return
		}

		err = th.Reverse()
		if internalError(w, err) {
			return
		}
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func TagHistoryHandler(w http.ResponseWriter, r *http.Request) {
	var p TagHistoryPage

	p.UserInfo = userCookies(w, r)
	p.History = DM.GetTagHistory(10, 0)

	preloadTagHistory(p.History)

	p.Base.Title = "Tag History"

	renderTemplate(w, "taghistory", p)
}

func preloadTagHistory(histories []*DM.TagHistory) {
	for _, h := range histories {
		for _, e := range h.QETags(DM.DB) {
			e.Tag.QID(DM.DB)
			e.Tag.QTag(DM.DB)
			e.Tag.QNamespace(DM.DB).QNamespace(DM.DB)
		}
		h.Post.QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFMime,
		)

		h.User.QName(DM.DB)
		h.User.QID(DM.DB)
		h.User.QFlag(DM.DB)
	}
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
