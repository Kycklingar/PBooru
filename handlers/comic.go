package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
)

type comicsPage struct {
	Comics     []*DM.Comic
	User       User
	UserInfo   UserInfo
	Pageinator Pageination
	Time       string
}

func ComicsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		if title == "" {
			http.Error(w, "Title cant be empty", http.StatusBadRequest)
			return
		}

		c := DM.NewComic()
		c.SetTitle(title)
		err := c.Save(DM.DB)
		if err != nil {
			log.Print(err)
			http.Error(w, "Oops", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/comic/%d", c.QID(DM.DB)), http.StatusSeeOther)
		return
	}
	splt := splitURI(r.RequestURI)
	var err error

	offset := 1
	if len(splt) >= 2 {
		offset, err = strconv.Atoi(splt[1])
		if err != nil {
			http.Error(w, "Invalid Path", http.StatusBadRequest)
			return
		}
	}
	bm := benchmark.Begin()

	var p comicsPage

	var cc DM.ComicCollector
	err = cc.Get(5, (offset-1)*5)
	if err != nil {
		http.Error(w, "Oops.", http.StatusInternalServerError)
		return
	}

	p.Comics = cc.Comics

	for _, c := range cc.Comics {
		c.QChapters(DM.DB)
		c.QChapterCount(DM.DB)
		c.QPageCount(DM.DB)
		c.QTitle(DM.DB)
		if len(c.Chapters) <= 0 {
			continue
		}
		c.Chapters[0].QID(DM.DB)
		c.Chapters[0].QOrder(DM.DB)
		c.Chapters[0].QTitle(DM.DB)
		c.Chapters[0].QPageCount(DM.DB)
		for _, p := range c.Chapters[0].QPosts(DM.DB) {
			p.QOrder(DM.DB)
			p.QID(DM.DB)
			p.Post.QID(DM.DB)
			p.Post.QHash(DM.DB)
			p.Post.QThumbnails(DM.DB)
		}
	}

	p.Pageinator = pageinate(cc.TotalComics, 5, offset, 10)

	var u *DM.User
	u, p.UserInfo = getUser(w, r)
	p.User = tUser(u)
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comics", p)
}

type comicPage struct {
	Base    base
	Comic   *DM.Comic
	Chapter *DM.Chapter

	User     User
	UserInfo UserInfo
	Full     bool
	Time     string
}

func ComicHandler(w http.ResponseWriter, r *http.Request) {
	var p comicPage
	bm := benchmark.Begin()
	spl := splitURI(r.URL.EscapedPath())
	if len(spl) < 2 {
		notFoundHandler(w, r)
		return
	}

	id, err := strconv.Atoi(spl[1])
	if err != nil {
		log.Print(err)
		notFoundHandler(w, r)
		return
	}

	p.Comic = DM.NewComic()

	p.Comic.SetID(id)
	p.Comic.QID(DM.DB)
	p.Comic.QTitle(DM.DB)
	p.Comic.QChapterCount(DM.DB)
	p.Base.Title = p.Comic.Title

	if len(spl) >= 3 {
		cOrd, err := strconv.Atoi(spl[2])
		if err != nil {
			log.Print(err)
			notFoundHandler(w, r)
			return
		}

		chptrs := p.Comic.QChapters(DM.DB)
		for _, chp := range chptrs {
			if chp.QOrder(DM.DB) == cOrd {
				p.Chapter = chp
				break
			}
		}
		if p.Chapter.ID == 0 {
			notFoundHandler(w, r)
			return
		}

		p.Chapter.QPageCount(DM.DB)
		p.Chapter.QTitle(DM.DB)
		p.Chapter.QOrder(DM.DB)
		for _, cp := range p.Chapter.QPosts(DM.DB) {
			cp.QID(DM.DB)
			cp.QOrder(DM.DB)
			cp.Post.QHash(DM.DB)
			cp.Post.QThumbnails(DM.DB)
			cp.Post.QID(DM.DB)
		}

		p.Base.Title += fmt.Sprint(" - C", p.Chapter.Order)
		if p.Chapter.Title != "" {
			p.Base.Title += " - " + p.Chapter.Title
		}
	} else {

		for _, c := range p.Comic.QChapters(DM.DB) {
			c.QPageCount(DM.DB)
			c.QTitle(DM.DB)
			c.QOrder(DM.DB)
			for _, cp := range c.QPosts(DM.DB) {
				cp.QID(DM.DB)
				cp.QOrder(DM.DB)
				cp.Post.QHash(DM.DB)
				cp.Post.QThumbnails(DM.DB)
				cp.Post.QID(DM.DB)
			}
		}
	}

	if len(r.FormValue("full")) > 0 {
		p.Full = true
	}

	var u *DM.User
	u, p.UserInfo = getUser(w, r)

	p.User = tUser(u)
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comic", p)
}

func ComicAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}
	u, _ := getUser(w, r)
	if u.QFlag(DM.DB) != DM.AdmFAdmin {
		http.Error(w, errMustBeAdmin, http.StatusBadRequest)
		return
	}
	cIDStr := r.FormValue("comicid")
	pIDStr := r.FormValue("postid")
	pOrderStr := r.FormValue("postorder")
	chIDStr := r.FormValue("chapterid")

	cID, err := strconv.Atoi(cIDStr)
	if err != nil {
		http.Error(w, "comicid not a number", http.StatusBadRequest)
		return
	}
	pID, err := strconv.Atoi(pIDStr)
	if err != nil {
		http.Error(w, "postid not integer", http.StatusBadRequest)
		return
	}
	pOrder, err := strconv.Atoi(pOrderStr)
	if err != nil {
		http.Error(w, "order not integer", http.StatusBadRequest)
		return
	}
	chID, err := strconv.Atoi(chIDStr)
	if err != nil {
		http.Error(w, "chapterid not integer", http.StatusBadRequest)
		return
	}

	c := DM.NewComic()
	err = c.SetID(cID)
	if err != nil {
		log.Print(err)
		http.Error(w, "Comic doesn't exist", http.StatusBadRequest)
		return
	}

	ch := c.Chapter(DM.DB, chID)
	if ch == nil {
		notFoundHandler(w, r)
		return
	}

	cp := ch.NewComicPost()
	err = cp.Post.SetID(DM.DB, pID)
	if err != nil {
		http.Error(w, "post doesn't exist", http.StatusBadRequest)
		return
	}

	cp.Order = pOrder

	err = cp.Save(DM.DB, true)
	if err != nil {
		log.Print(err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func EditComicHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	user, _ := getUser(w, r)
	if user.QFlag(DM.DB) != DM.AdmFAdmin {
		http.Error(w, errMustBeAdmin, http.StatusBadRequest)
		return
	}

	if r.FormValue("newchapter") == "true" {
		cIDStr := r.FormValue("comicid")
		title := r.FormValue("title")
		orderStr := r.FormValue("order")

		cID, err := strconv.Atoi(cIDStr)
		if err != nil {
			http.Error(w, "comicid not integer", http.StatusBadRequest)
			return
		}

		order, err := strconv.Atoi(orderStr)
		if err != nil {
			http.Error(w, "order not integer", http.StatusBadRequest)
			return
		}

		if order < 0 {
			http.Error(w, "order must be positive", http.StatusBadRequest)
			return
		}

		c := DM.NewComic()
		c.SetID(cID)

		ch := c.NewChapter()
		ch.Title = title
		ch.Order = order

		err = ch.Save(DM.DB)
		if err != nil {
			log.Print(err)
			http.Error(w, "Oops", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/comic/%d/%d", cID, order), http.StatusSeeOther)
	} else if r.FormValue("editpost") == "true" {
		cIDStr := r.FormValue("comicid")
		pIDStr := r.FormValue("postid")
		orderStr := r.FormValue("order")
		chIDStr := r.FormValue("chapterid")

		cID, err := strconv.Atoi(cIDStr)
		if err != nil {
			http.Error(w, "comicid not integer", http.StatusBadRequest)
			return
		}
		pID, err := strconv.Atoi(pIDStr)
		if err != nil {
			http.Error(w, "postid not integer", http.StatusBadRequest)
			return
		}
		order, err := strconv.Atoi(orderStr)
		if err != nil {
			http.Error(w, "order not integer", http.StatusBadRequest)
			return
		}
		chID, err := strconv.Atoi(chIDStr)
		if err != nil {
			http.Error(w, "chapterid not integer", http.StatusBadRequest)
			return
		}

		c := DM.NewComic()
		if err = c.SetID(cID); err != nil {
			log.Print(err)
			http.Error(w, "comic doesn't exist", http.StatusBadRequest)
			return
		}

		ch := c.Chapter(DM.DB, chID)
		if ch == nil {
			http.Error(w, "Chapter doesn't exist", http.StatusBadRequest)
			return
		}

		cp := ch.NewComicPost()
		cp.Post.SetID(DM.DB, pID)
		cp.Order = order

		err = cp.Save(DM.DB, true)
		if err != nil {
			log.Print(err)
			http.Error(w, "Oops", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/comic/%d/%d", cID, cp.Chapter.QOrder(DM.DB)), http.StatusSeeOther)
	}
}
