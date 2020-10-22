package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	DM "github.com/kycklingar/PBooru/DataManager"
)

type catalog struct {
	Threads []DM.Thread
	Board string
}

type thread struct {
	Board string
	Thread int
	Replies []DM.ForumPost
}

func boardHandler(w http.ResponseWriter, r *http.Request) {
	uri := uriSplitter(r)

	board, err := uri.getAtIndex(1)
	if err != nil {
		boardsHandler(w, r)
		return
	}

	res, _ := uri.getAtIndex(2)
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusBadRequest)
	//	return
	//}

	switch res {
		case "thread":
			threadHandler(board, w, r)
		default:
			catalogHandler(board, w, r)
	}

}

func boardsHandler(w http.ResponseWriter, r *http.Request) {
	page := struct {
		User *DM.User
		Boards map[string][]DM.Board
		Categories []string
	}{}

	var err error

	page.Boards, err = DM.GetBoards()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page.User, _ = getUser(w, r)
	if page.User.QFlag(DM.DB).Special() {
		page.Categories, err = DM.GetCategories()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	renderTemplate(w, "forum", page)
}

func catalogHandler(board string, w http.ResponseWriter, r *http.Request) {
	var cat catalog
	var err error
	cat.Board = board

	cat.Threads, err = DM.GetCatalog(cat.Board)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "catalog", cat)
}

func threadHandler(board string, w http.ResponseWriter, r *http.Request) {
	var (
		th = thread{Board:board}
		err error
	)

	uri := uriSplitter(r)

	th.Thread, err = uri.getIntAtIndex(3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	th.Replies, err = DM.GetThread(board, th.Thread)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "thread", th)
}

func newThreadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost{
		notFoundHandler(w, r)
		return
	}

	var reply *int
	if i, err := strconv.Atoi(r.FormValue("reply-to")); err == nil {
		reply = new(int)
		*reply = i
	}

	title := r.FormValue("title")
	body := r.FormValue("body")
	board := r.FormValue("board")
	rid, err := DM.NewForumPost(reply, board, title, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/forum/%s/thread/%d", board, rid), http.StatusSeeOther)
}

func newCategoryHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	name := r.FormValue("name")

	if err := DM.NewCategory(name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/forum/", http.StatusSeeOther)
}

func newBoardHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	name := r.FormValue("name")
	uri := r.FormValue("uri")
	description := r.FormValue("description")
	category := r.FormValue("category")

	if err := DM.NewBoard(uri, name, description, category); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/forum/", http.StatusSeeOther)
}
