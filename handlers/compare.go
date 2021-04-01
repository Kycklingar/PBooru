package handlers

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

type comparisonPage struct {
	UserInfo UserInfo

	Posts   []*DM.Post
	Removed []*DM.Post

	Report int
}

func colorID(id int) string {
	n := strconv.Itoa(id)
	h := sha256.Sum256([]byte(n))
	return fmt.Sprintf("%x", h[:3])
}

//func colorID(id int) string {
//	runes := []rune("0123456789ABCDEF")
//	var cid string
//	var m = id * 77777
//	for i := 1; i <= 6; i++ {
//		cid += string(runes[i*m%len(runes)])
//	}
//	return fmt.Sprint(cid)
//}

func (c comparisonPage) ColorID(id int) string {
	return colorID(id)
}

func comparisonHandler(w http.ResponseWriter, r *http.Request) {
	var page comparisonPage

	_, page.UserInfo = getUser(w, r)

	r.ParseForm()

	in := func(id int, posts []*DM.Post) bool {
		for _, p := range posts {
			if p.ID == id {
				return true
			}
		}

		return false
	}

	for _, id := range r.Form["removed-id"] {
		var err error
		p := DM.NewPost()
		p.ID, err = strconv.Atoi(id)
		if err != nil {
			log.Println(err)
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		if !in(p.ID, page.Removed) {
			page.Removed = append(page.Removed, p)
		}
	}

	for _, id := range r.Form["post-id"] {
		var err error
		var p = DM.NewPost()
		p.ID, err = strconv.Atoi(id)
		if err != nil {
			log.Println(err)
			http.Error(w, "Invalid post ID", http.StatusBadRequest)
			return
		}

		if !in(p.ID, page.Posts) && !in(p.ID, page.Removed) {
			page.Posts = append(page.Posts, p)
		}
	}


	for i := range page.Posts {
		page.Posts[i] = DM.CachedPost(page.Posts[i])

		page.Posts[i].QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFDimension,
			DM.PFSize,
			DM.PFRemoved,
		)
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

		page.Posts[i].QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFDimension,
			DM.PFSize,
			DM.PFRemoved,
		)
	}

	renderTemplate(w, "compare2", page)
}
