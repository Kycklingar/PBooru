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
	page.EditMode = len(r.Form["edit"]) > 0

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

func verifyInteger(r *http.Request, formKey... string) (m map[string]int, err error) {
	m = make(map[string]int)
	for _, key := range formKey {
		var v int
		v, err = strconv.Atoi(r.FormValue(key))
		if err != nil {
			return
		}
		m[key] = v
	}

	return
}

func comicAddChapterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		comicIDKey = "comic-id"
		titleKey = "title"
		orderKey = "order"
	)

	m, err := verifyInteger(r, comicIDKey, orderKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chapter := DM.NewChapter()
	chapter.Comic = DM.NewComic()
	chapter.Comic.ID = m[comicIDKey]
	chapter.Order = m[orderKey]
	chapter.Title = r.FormValue(titleKey)
	if err = chapter.Save(DM.DB, user); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/comic/%d/%d/", chapter.Comic.ID, chapter.Order), http.StatusSeeOther)
}

func comicAddPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		postKey = "post-id"
		postOrderKey = "order"
		chapterIdKey = "chapter-id"
	)

	m, err := verifyInteger(r, postKey, postOrderKey, chapterIdKey)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cp := DM.NewComicPost()

	cp.Post = DM.NewPost()
	cp.Post.ID = m[postKey]

	cp.Chapter = DM.NewChapter()
	cp.Chapter.ID = m[chapterIdKey]

	cp.Order = m[postOrderKey]

	if err = cp.Save(user, false); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func comicEditPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		cpIDKey = "cp-id"
		postOrderKey = "order"
		chapterIdKey = "chapter-id"
	)

	m, err := verifyInteger(r, cpIDKey, postOrderKey, chapterIdKey)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cp := DM.NewComicPost()
	cp.ID = m[cpIDKey]

	cp.Chapter = DM.NewChapter()
	cp.Chapter.ID = m[chapterIdKey]
	cp.Order = m[postOrderKey]

	if err = cp.Save(user, true); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

