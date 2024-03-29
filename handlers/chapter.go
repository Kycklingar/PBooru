package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/user"
)

func renderChapter(comic *DM.Comic, chapterIndex int, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var page struct {
		Base        base
		Comic       *DM.Comic
		Chapter     *DM.Chapter
		UserInfo    UserInfo
		User        user.User
		Full        bool
		EditMode    bool
		AddPostMode bool
		Time        string
	}

	page.User, page.UserInfo = getUser(w, r)

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
	if !user.Flag.Comics() {
		permError(w, "Comics")
		return
	}

	const (
		chapterIDKey = "chapter-id"
		titleKey     = "title"
		orderKey     = "order"
	)

	m, err := verifyInteger(r, chapterIDKey, orderKey)
	if badRequest(w, err) {
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.EditChapter(m[chapterIDKey], m[orderKey], r.FormValue(titleKey)))
	err = ua.Exec()
	if internalError(w, err) {
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
	if !user.Flag.Comics() {
		permError(w, "Comics")
		return
	}

	const (
		chapterIDKey = "chapter-id"
		byKey        = "by"
		symbolKey    = "symbol"
		pageKey      = "page"
	)

	m, err := verifyInteger(r, chapterIDKey, byKey, symbolKey, pageKey)
	if badRequest(w, err) {
		return
	}

	ua := DM.UserAction(user)
	ua.Add(
		DM.ShiftChapterPages(
			m[chapterIDKey],
			m[byKey],
			m[symbolKey],
			m[pageKey],
		),
	)
	err = ua.Exec()
	if internalError(w, err) {
		return
	}

	toRefOrOK(w, r, "Pages shifted successfully!")
}

func deleteChapterHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.Flag.Special() {
		permError(w, "Special")
		return
	}

	chapterID, err := strconv.Atoi(r.FormValue("chapter-id"))
	if badRequest(w, err) {
		return
	}

	var comicID int
	ua := DM.UserAction(user)
	ua.Add(DM.DeleteChapter(chapterID, &comicID))
	err = ua.Exec()
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/comic/%d/", comicID), http.StatusSeeOther)
}

func addComicPageHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.Flag.Comics() {
		permError(w, "Comics")
		return
	}

	const (
		chapterIDKey = "chapter-id"
		postIDKey    = "post-id"
		pageKey      = "page"
	)

	m, err := verifyInteger(r, chapterIDKey, postIDKey, pageKey)
	if badRequest(w, err) {
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.CreateComicPage(m[chapterIDKey], m[postIDKey], m[pageKey]))
	err = ua.Exec()
	if internalError(w, err) {
		return
	}

	toRefOrOK(w, r, "Comic page has been added")
}

func editComicPageHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.Flag.Comics() {
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
	if badRequest(w, err) {
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.EditComicPage(m[pageIDKey], m[chapterIDKey], m[postIDKey], m[pageKey]))
	err = ua.Exec()
	if internalError(w, err) {
		return
	}

	toRefOrOK(w, r, "Comic page has been edited")
}

func deleteComicPageHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.Flag.Comics() {
		permError(w, "Comics")
		return
	}

	pageID, err := strconv.Atoi(r.FormValue("page-id"))
	if badRequest(w, err) {
		return
	}

	ua := DM.UserAction(user)
	ua.Add(DM.DeleteComicPage(pageID))
	err = ua.Exec()
	if internalError(w, err) {
		return
	}

	toRefOrOK(w, r, "Comic page has been deleted")
}
