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

	Report int
}

func colorID(id int) string {
	runes := []rune("0123456789ABCDEF")
	var cid string
	var m = id * 77777
	for i := 1; i <= 6; i++ {
		cid += string(runes[i*m%len(runes)])
	}
	return fmt.Sprint(cid)
}

func (c comparisonPage) ColorID(id int) string {
	return colorID(id)
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
