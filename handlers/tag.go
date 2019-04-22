package handlers

import (
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
)

type Sidebar struct {
	TotalPosts int
	Tags       []*DM.Tag
	Query      string
	Filter     string
	Unless     string
}

const tagLimit = 200

func TagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		type P struct {
			Query     string
			Tags      []*DM.Tag
			Tag       *DM.Tag
			To        *DM.Tag
			From      []*DM.Tag
			Parents   []*DM.Tag
			Children  []*DM.Tag
			Paginator Pageination
		}
		var p P

		tagStr := r.FormValue("tag")
		currPage := 1
		var err error
		f := splitURI(r.URL.Path)
		if len(f) >= 2 && f[1] != "" {
			currPage, err = strconv.Atoi(f[1])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		bm := benchmark.Begin()
		var tc DM.TagCollector

		if len(tagStr) > 0 {
			tc.Parse(tagStr)
			tc = tc.SuggestedTags(DM.DB)
			p.Query = "?tag=" + tagStr
		} else {
			err = tc.Get(tagLimit, (currPage-1)*tagLimit)
		}

		for _, t := range tc.Tags {
			t.QNamespace(DM.DB).QNamespace(DM.DB)
		}
		if err != nil {
			http.Error(w, "Oops", http.StatusInternalServerError)
			log.Print(err)
			return
		}

		p.Tags = tc.Tags

		if len(f) >= 3 && f[2] != "" {
			tagID, err := strconv.Atoi(f[2])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
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

			p.To = a.QTo(DM.DB)
			preload(p.To)

			p.Parents = p.Tag.Parents(DM.DB)
			preload(p.Parents...)

			p.Children = p.Tag.Children(DM.DB)
			preload(p.Children...)

			bm.End(performBenchmarks)
		}

		p.Paginator = pageinate(tc.Total(), tagLimit, currPage, 30)

		renderTemplate(w, "tags", p)
		return
	}

	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Tags() {
		http.Error(w, "Insufficient privileges. Want \"Tags\"", http.StatusForbidden)
		return
	}

	if r.FormValue("action") == "alias" {
		from := r.FormValue("from")
		to := r.FormValue("to")

		alias := DM.NewAlias()

		tc := DM.TagCollector{}
		if err := tc.Parse(from); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		alias.Tag = tc.Tags[0]
		tc = DM.TagCollector{}
		if err := tc.Parse(to); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		alias.To = tc.Tags[0]

		err := alias.Save(DM.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else if r.FormValue("action") == "parent" {
		child := r.FormValue("child")
		parent := r.FormValue("parent")

		c := DM.NewTag()
		c.Parse(child)

		p := DM.NewTag()
		p.Parse(parent)
		tx, err := DM.DB.Begin()
		if err != nil {
			log.Println(err)
			http.Error(w, ErrInternal, http.StatusInternalServerError)
			return
		}
		if err = c.AddParent(tx, p); err != nil {
			log.Println(err)
			http.Error(w, ErrInternal, http.StatusInternalServerError)
			tx.Rollback()
			return
		}
		if err = tx.Commit(); err != nil {
			log.Println(err)
			http.Error(w, ErrInternal, http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/tags/", http.StatusSeeOther)
}

type TagHistoryPage struct {
	Base       base
	History    []*DM.TagHistory
	UserInfo   UserInfo
	Pageinator Pageination
}

func TagHistoryHandler(w http.ResponseWriter, r *http.Request) {
	var p TagHistoryPage

	p.UserInfo = userCookies(w, r)
	p.History = DM.GetTagHistory(10, 0)
	for _, h := range p.History {
		for _, e := range h.QETags(DM.DB) {
			e.Tag.QID(DM.DB)
			e.Tag.QTag(DM.DB)
			e.Tag.QNamespace(DM.DB).QNamespace(DM.DB)
		}
		h.Post.QID(DM.DB)
		h.Post.QHash(DM.DB)
		h.Post.QThumbnails(DM.DB)
		h.Post.QMime(DM.DB).QName(DM.DB)
		h.Post.QMime(DM.DB).QType(DM.DB)

		h.User.QName(DM.DB)
		h.User.QID(DM.DB)
		h.User.QFlag(DM.DB)
	}
	p.Base.Title = "Tag History"

	renderTemplate(w, "taghistory", p)
}
