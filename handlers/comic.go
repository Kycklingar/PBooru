package handlers

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	Edit       bool

	Query values
}

type values map[string]string

func (v values) Encode() string {
	if v == nil || len(v) <= 0 {
		return ""
	}

	var out string
	for k, v := range v {
		if len(out) > 0 {
			out += "&"
		}
		out += url.QueryEscape(k) + "=" + url.QueryEscape(v)
	}

	return "?" + out
}

func (v values) AddEncode(key, val string) string {
	nv := make(values)
	for k, ov := range v {
		nv[k] = ov
	}

	nv[key] = val
	return nv.Encode()
}

func vals(val url.Values) values {
	var va = make(values)

	for k, v := range val {
		if len(v) > 0 && len(v[0]) > 0 {
			va[k] = v[0]
		}
	}

	return va
}

const comicsPerPage = 25

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
		err := c.Save(user)
		if err != nil {
			log.Print(err)
			http.Error(w, "Oops", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/comic/%d", c.QID(DM.DB)), http.StatusSeeOther)
		return
	}
	var err error

	uri := uriSplitter(r)

	offset := 1
	if uri.length() >= 2 {
		offset, err = uri.getIntAtIndex(1)
		if err != nil {
			http.Error(w, "Invalid Path", http.StatusBadRequest)
			return
		}
	}
	bm := benchmark.Begin()

	var p comicsPage

	if err = r.ParseForm(); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	p.Edit = len(r.Form["edit"]) >= 1

	var cc DM.ComicCollector

	p.Query = vals(r.Form)

	if len(p.Query["tags"]) > 0 {
		p.Query["tags"] += ", " + p.Query["append-tag"]
	} else if len(p.Query["append-tag"]) > 0 {
		p.Query["tags"] += p.Query["append-tag"]
	}
	delete(p.Query, "append-tag")

	err = cc.Search(r.FormValue("title"), p.Query["tags"], comicsPerPage, (offset-1)*comicsPerPage)
	if err != nil {
		log.Println(err)
		http.Error(w, "Oops.", http.StatusInternalServerError)
		return
	}

	p.Comics = cc.Comics

	for _, c := range cc.Comics {
		c.QTagSummary(DM.DB)
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
		for i, p := range c.Chapters[0].QPosts(DM.DB) {
			if i >= 5 {
				break
			}
			p.QOrder(DM.DB)
			p.QID(DM.DB)
			p.Post.QMul(
				DM.DB,
				DM.PFHash,
				DM.PFThumbnails,
			)
		}
	}

	p.Pageinator = pageinate(cc.TotalComics, comicsPerPage, offset, 10)

	var u *DM.User
	u, p.UserInfo = getUser(w, r)
	u.QFlag(DM.DB)
	p.User = u
	p.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comics", p)
}

func comicHandler(w http.ResponseWriter, r *http.Request) {
	page := struct {
		Base     base
		Comic    *DM.Comic
		User     *DM.User
		UserInfo UserInfo
		EditMode bool
	}{}

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)

	uri := uriSplitter(r)

	if uri.length() >= 3 {
		chapterHandler(w, r)
		return
	}

	comicID, err := uri.getIntAtIndex(1)
	if err != nil {
		notFoundHandler(w)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Println(err)
	}

	page.EditMode = len(r.Form["edit-mode"]) > 0

	if page.Comic, err = DM.NewComicByID(comicID); err != nil {
		log.Println(err)
		notFoundHandler(w)
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
			cpost.Post.QMul(
				DM.DB,
				DM.PFHash,
				DM.PFThumbnails,
			)
		}
	}

	renderTemplate(w, "comic", page)
}

func comicDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		comicIDKey = "comic-id"
	)
	m, err := verifyInteger(r, comicIDKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := DM.NewComic()
	c.ID = m[comicIDKey]

	if err = c.Delete(user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/comics/", http.StatusSeeOther)
}

func comicUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		comicIDKey = "comic-id"
		titleKey   = "title"
	)

	m, err := verifyInteger(r, comicIDKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	comic, err := DM.NewComicByID(m[comicIDKey])
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	comic.Title = r.FormValue(titleKey)
	if err = comic.SaveEdit(user); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func chapterHandler(w http.ResponseWriter, r *http.Request) {
	bm := benchmark.Begin()
	uri := uriSplitter(r)
	if uri.length() > 3 {
		notFoundHandler(w)
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
		Base        base
		Chapter     *DM.Chapter
		UserInfo    UserInfo
		User        *DM.User
		Full        bool
		EditMode    bool
		AddPostMode bool
		Time        string
	}{}

	if err := r.ParseForm(); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page.Full = len(r.Form["full"]) > 0
	page.EditMode = len(r.Form["edit-mode"]) > 0
	page.AddPostMode = len(r.Form["add-mode"]) > 0

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)

	page.Chapter = comic.Chapter(DM.DB, chapterIndex)
	if page.Chapter == nil {
		notFoundHandler(w)
		return
	}

	page.Chapter.QTitle(DM.DB)
	page.Chapter.QOrder(DM.DB)
	page.Chapter.QPageCount(DM.DB)
	page.Chapter.QPosts(DM.DB)

	for _, cpost := range page.Chapter.Posts {
		cpost.QOrder(DM.DB)
		cpost.QPost(DM.DB)
		cpost.Post.QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
		)
	}

	page.Base.Title += fmt.Sprint(page.Chapter.Comic.Title, " - C", page.Chapter.Order)
	if page.Chapter.Title != "" {
		page.Base.Title += " - " + page.Chapter.Title
	}

	page.Time = bm.EndStr(performBenchmarks)

	renderTemplate(w, "comic_chapter", page)
}

func verifyInteger(r *http.Request, formKey ...string) (m map[string]int, err error) {
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
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		comicIDKey = "comic-id"
		titleKey   = "title"
		orderKey   = "order"
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/comic/%d/%d/", chapter.Comic.ID, chapter.Order), http.StatusSeeOther)
}

func comicEditChapterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		chapterIDKey = "chapter-id"
		titleKey     = "title"
		orderKey     = "order"
	)

	m, err := verifyInteger(r, chapterIDKey, orderKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chapter := DM.NewChapter()
	chapter.ID = m[chapterIDKey]
	chapter.QComic(DM.DB)
	chapter.Order = m[orderKey]
	chapter.Title = r.FormValue(titleKey)
	if err = chapter.SaveEdit(DM.DB, user); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func chapterShiftHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		chapterIDKey = "chapter-id"
		byKey        = "by"
		symbolKey    = "symbol"
		pageKey      = "page"
	)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	m, err := verifyInteger(r, chapterIDKey, byKey, symbolKey, pageKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chapter := DM.NewChapter()
	chapter.ID = m[chapterIDKey]

	err = chapter.ShiftPosts(user, m[symbolKey], m[pageKey], m[byKey])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func comicRemoveChapterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		chapterIDKey = "chapter-id"
	)

	m, err := verifyInteger(r, chapterIDKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chapter := DM.NewChapter()
	chapter.ID = m[chapterIDKey]

	if err = chapter.Delete(user); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func comicAddPageApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	post, err := postFromForm(r)
	if err != nil {
		if err == errNoID {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	const (
		postOrderKey = "order"
		chapterIdKey = "chapter-id"
	)

	m, err := verifyInteger(r, postOrderKey, chapterIdKey)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}


	cp := DM.NewComicPost()
	cp.Post = post
	cp.Chapter = DM.NewChapter()
	cp.Chapter.ID = m[chapterIdKey]

	cp.Order = m[postOrderKey]

	if err = cp.Save(user, false); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func comicAddPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		postKey      = "post-id"
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

	r.Form.Add("add-mode", "")
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func comicEditPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		cpIDKey      = "cp-id"
		postOrderKey = "order"
		postIDKey    = "post-id"
		chapterIdKey = "chapter-id"
	)

	m, err := verifyInteger(r, cpIDKey, postOrderKey, postIDKey, chapterIdKey)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cp := DM.NewComicPost()
	cp.ID = m[cpIDKey]
	cp.Post = DM.NewPost()
	cp.Post.ID = m[postIDKey]

	cp.Chapter = DM.NewChapter()
	cp.Chapter.ID = m[chapterIdKey]
	cp.Order = m[postOrderKey]

	if err = cp.SaveEdit(user); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func comicDeletePageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		http.Error(w, lackingPermissions("Comics"), http.StatusBadRequest)
		return
	}

	const (
		cpIDKey = "cp-id"
	)

	m, err := verifyInteger(r, cpIDKey)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cp := DM.NewComicPost()
	cp.ID = m[cpIDKey]

	if err = cp.Delete(user); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}
