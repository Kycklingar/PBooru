package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

type comparisonPage struct {
	UserInfo UserInfo

	Posts []*DM.Post
}

func (c comparisonPage) ColorID(id int) string {
	runes := []rune("0123456789ABCDEF")
	var cid string
	var m = id * 77777
	for i := 1; i <= 6; i++ {
		cid += string(runes[i*m%len(runes)])
	}
	return fmt.Sprint(cid)
}

func comparisonHandler(w http.ResponseWriter, r *http.Request) {
	var page comparisonPage

	_, page.UserInfo = getUser(w, r)

	r.ParseForm()

	for _, id := range r.Form["post-id"] {
		var err error
		var p = DM.NewPost()
		p.ID, err = strconv.Atoi(id)
		if err != nil {
			log.Println(err)
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		page.Posts = append(page.Posts, p)
	}

	for i := range page.Posts {
		page.Posts[i] = DM.CachedPost(page.Posts[i])

		page.Posts[i].QHash(DM.DB)
		page.Posts[i].QThumbnails(DM.DB)
		page.Posts[i].QMime(DM.DB).QName(DM.DB)
		page.Posts[i].QMime(DM.DB).QType(DM.DB)

		page.Posts[i].QDimensions(DM.DB)
		page.Posts[i].QSize(DM.DB)
	}

	renderTemplate(w, "compare", page)
}

func compare2Handler(w http.ResponseWriter, r *http.Request) {
	var page comparisonPage

	_, page.UserInfo = getUser(w, r)

	r.ParseForm()

	for _, id := range r.Form["post-id"] {
		var err error
		var p = DM.NewPost()
		p.ID, err = strconv.Atoi(id)
		if err != nil {
			log.Println(err)
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		page.Posts = append(page.Posts, p)
	}

	for i := range page.Posts {
		page.Posts[i] = DM.CachedPost(page.Posts[i])

		page.Posts[i].QHash(DM.DB)
		page.Posts[i].QThumbnails(DM.DB)
		page.Posts[i].QMime(DM.DB).QName(DM.DB)
		page.Posts[i].QMime(DM.DB).QType(DM.DB)

		page.Posts[i].QDimensions(DM.DB)
		page.Posts[i].QSize(DM.DB)
	}

	renderTemplate(w, "compare2", page)
}

func reportDuplicatesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	var note = r.FormValue("note")
	user, _ := getUser(w, r)

	const bestPostIDKey = "best-id"

	m, err := verifyInteger(r, bestPostIDKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var drposts []DM.DRPost

	for _, idstr := range r.Form["post-ids"] {
		var rp DM.DRPost
		rp.Post = DM.NewPost()

		rp.Post.ID, err = strconv.Atoi(idstr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if rp.Post.ID == m[bestPostIDKey] {
			rp.Score = 1
		}

		drposts = append(drposts, rp)
	}

	if err = DM.ReportDuplicates(drposts, user, note); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
