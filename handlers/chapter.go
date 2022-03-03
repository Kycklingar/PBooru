package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func renderChapter(comic *DM.Comic, chapterIndex int, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var page struct {
		Base        base
		Comic       *DM.Comic
		Chapter     *DM.Chapter
		UserInfo    UserInfo
		User        *DM.User
		Full        bool
		EditMode    bool
		AddPostMode bool
		Time        string
	}

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)

	page.EditMode = len(r.Form["edit-mode"]) > 0
	page.Full = len(r.Form["full"]) > 0

	page.Comic = comic
	page.Chapter = page.Comic.ChapterIndex(chapterIndex)
	if page.Chapter == nil {
		notFoundHandler(w)
		return
	}

	renderTemplate(w, "chapter", page)
}

func editChapterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Comics() {
		permError(w, "Comics")
		return
	}

	const (
		chapterIDKey = "chapter-id"
		titleKey     = "title"
		orderKey     = "order"
	)

	m, err := verifyInteger(r, chapterIDKey, orderKey)
	if err != nil {
		badRequest(w, err)
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.EditChapter(m[chapterIDKey], m[orderKey], r.FormValue(titleKey)))
	err = ua.Exec()
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func deleteChapterHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Comics() {
		permError(w, "Comics")
		return
	}

	chapterID, err := strconv.Atoi(r.FormValue("chapter-id"))
	if err != nil {
		badRequest(w, err)
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.DeleteChapter(chapterID))
	err = ua.Exec()
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func addComicPageHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Comics() {
		permError(w, "Comics")
		return
	}

	const (
		chapterIDKey = "chapter-id"
		postIDKey    = "post-id"
		pageKey      = "page"
	)

	m, err := verifyInteger(r, chapterIDKey, postIDKey, pageKey)
	if err != nil {
		badRequest(w, err)
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.CreateComicPage(m[chapterIDKey], m[postIDKey], m[pageKey]))
	err = ua.Exec()
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func editComicPageHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Comics() {
		permError(w, "Comics")
		return
	}

	const (
		pageIDKey    = "page-id"
		chapterIDKey = "chapter-id"
		postIDKey    = "post-id"
		pageKey      = "page"
	)

	m, err := verifyInteger(r, pageIDKey, chapterIDKey, postIDKey, pageKey)
	if err != nil {
		badRequest(w, err)
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.EditComicPage(m[pageIDKey], m[chapterIDKey], m[postIDKey], m[pageKey]))
	err = ua.Exec()
	if err != nil {
		log.Println(err)
		internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func deleteComicPageHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Comics() {
		permError(w, "Comics")
		return
	}

	pageID, err := strconv.Atoi(r.FormValue("page-id"))
	if err != nil {
		badRequest(w, err)
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.DeleteComicPage(pageID))
	err = ua.Exec()
	if err != nil {
		internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
