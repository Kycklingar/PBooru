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
	User       *DM.User
	UserInfo   UserInfo
	Pageinator Pageination
	Time       string
}

func ComicsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user, _ := getUser(w, r)
		if !user.QFlag(DM.DB).Comics() {
			http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
			return
		}

		title := r.FormValue("title")
		if title == "" {
			http.Error(w, "Title cant be empty", http.StatusBadRequest)
			return
		}

		c := DM.NewComic()
		c.Title = title
		err := c.Save(DM.DB, user)
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
	u.QFlag(DM.DB)
	p.User = u
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comics", p)
}

func comicHandler(w http.ResponseWriter, r *http.Request) {
	uri := uriSplitter(r)

	if uri.length() >= 3 {
		chapterHandler(w, r)
		return
	}

	comicID, err := uri.getIntAtIndex(1)
	if err != nil {
		notFoundHandler(w, r)
	}

	page := struct {
		Base  base
		Comic *DM.Comic
		User  *DM.User
		UserInfo UserInfo
	}{}

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)

	if page.Comic, err = DM.NewComicByID(comicID); err != nil {
		log.Println(err)
		notFoundHandler(w, r)
		return
	}

	page.Comic.ID = comicID
	page.Comic.QTitle(DM.DB)
	page.Comic.QChapters(DM.DB)

	page.Base.Title = page.Comic.Title

	const postsPerChapter = 5

	for _, chapter := range page.Comic.Chapters {
		chapter.QPosts(DM.DB)
		chapter.QTitle(DM.DB)
		chapter.QPageCount(DM.DB)
		for _, cpost := range chapter.PostsLimit(postsPerChapter) {
			cpost.QPost(DM.DB)
			cpost.Post.QHash(DM.DB)
		}
	}

	renderTemplate(w, "comic", page)
}

func chapterHandler(w http.ResponseWriter, r *http.Request) {
	bm := benchmark.Begin()
	uri := uriSplitter(r)
	if uri.length() > 3 {
		notFoundHandler(w, r)
		return
	}

	comicID, err := uri.getIntAtIndex(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chapterIndex, err := uri.getIntAtIndex(2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	comic, err := DM.NewComicByID(comicID)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	comic.QTitle(DM.DB)
	comic.QChapters(DM.DB)

	page := struct {
		Base base
		Chapter *DM.Chapter
		UserInfo UserInfo
		User *DM.User
		Full bool
		EditMode bool
		Time string
	}{}

	if err := r.ParseForm(); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page.Full = len(r.Form["full"]) > 0
	page.EditMode = len(r.Form["edit-mode"]) > 0

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)

	page.Chapter = comic.Chapter(DM.DB, chapterIndex)
	if page.Chapter == nil {
		notFoundHandler(w, r)
		return
	}

	page.Chapter.QTitle(DM.DB)
	page.Chapter.QOrder(DM.DB)
	page.Chapter.QPageCount(DM.DB)
	page.Chapter.QPosts(DM.DB)

	for _, cpost := range page.Chapter.Posts {
		cpost.QOrder(DM.DB)
		cpost.QPost(DM.DB)
		cpost.Post.QHash(DM.DB)
	}

	page.Base.Title += fmt.Sprint(page.Chapter.Comic.Title, " - C", page.Chapter.Order)
	if page.Chapter.Title != "" {
		page.Base.Title += " - " + page.Chapter.Title
	}

	page.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comic_chapter", page)
}

type comicPage struct {
	Base    base
	Comic   *DM.Comic
	Chapter *DM.Chapter

	User     *DM.User
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

	p.Comic.ID = id
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
	u.QFlag(DM.DB)

	p.User = u
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comic_chapter", p)
}

func ComicAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}
	u, _ := getUser(w, r)
	if !u.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
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
	c.ID = cID

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

	err = cp.Save(u, true)
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
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
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

		ch := DM.NewChapter()
		ch.Title = title
		ch.Order = order
		ch.Comic = DM.NewComic()
		ch.Comic.ID = cID

		err = ch.Save(DM.DB, user)
		if err != nil {
			log.Print(err)
			http.Error(w, "Oops", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/comic/%d/%d", cID, order), http.StatusSeeOther)
	} else if r.FormValue("editpost") == "true" {
		order, err := strconv.Atoi(r.FormValue("order"))
		if err != nil {
			http.Error(w, "order not integer", http.StatusBadRequest)
			return
		}
		chID, err := strconv.Atoi(r.FormValue("chapter-id"))
		if err != nil {
			http.Error(w, "chapter-id not integer", http.StatusBadRequest)
			return
		}

		cpID, err := strconv.Atoi(r.FormValue("cp-id"))
		if err != nil {
			http.Error(w, "cp-id not integer", http.StatusBadRequest)
			return
		}

		cp := DM.NewComicPost()
		cp.ID = cpID
		cp.QChapter(DM.DB)
		cp.Chapter.ID = chID
		cp.Order = order

		err = cp.Save(user, true)
		if err != nil {
			log.Print(err)
			http.Error(w, "Oops", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprint(r.Referer(), "../", chID), http.StatusSeeOther)
	}
}
