package handlers

import (
	"log"
	"net/http"
	"net/url"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
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
			tc.Parse(tagStr, ",")
			tc = tc.SuggestedTags(DM.DB)
			p.Query = "?tag=" + tagStr
		} else {
			err = tc.Get(tagLimit, (currPage-1)*tagLimit)
			if err != nil {
				http.Error(w, "Oops", http.StatusInternalServerError)
				log.Print(err)
				return
			}
		}

		for _, t := range tc.Tags {
			t.QTag(DM.DB)
			t.QNamespace(DM.DB).QNamespace(DM.DB)
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

			p.To, err = a.QTo(DM.DB)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
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
		if err := tc.Parse(from, ","); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		alias.Tag = tc.Tags[0]
		tc = DM.TagCollector{}
		if err := tc.Parse(to, ","); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		alias.To = tc.Tags[0]

		err := alias.Save()
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
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = th.Reverse()
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFMime,
		)

		h.User.QName(DM.DB)
		h.User.QID(DM.DB)
		h.User.QFlag(DM.DB)
	}
}
