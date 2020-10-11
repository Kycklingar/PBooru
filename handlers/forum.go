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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
