package apiv1

import (
	"errors"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func similarPostsHandler(w http.ResponseWriter, r *http.Request) {
	postIDstr := r.FormValue("id")
	postID, err := strconv.Atoi(postIDstr)
	if err != nil {
		apiError(w, errors.New("valid id required"), http.StatusBadRequest)
		return
	}

	var combineTags bool
	if len(r.FormValue("combTagNamespace")) > 0 {
		combineTags = true
	}

	type page struct {
		Posts []post
	}

	var p page

	dpost := DM.NewPost()
	if err = dpost.SetID(DM.DB, postID); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	var posts []*DM.Post
	if posts, err = dpost.FindSimilar(DM.DB, 5, false); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	for _, pst := range posts {
		var ps post
		if ps, err = dmToAPIPost(pst, nil, combineTags); err != nil {
			apiError(w, err, http.StatusInternalServerError)
			return
		}
		p.Posts = append(p.Posts, ps)
	}
	writeJson(w, p)
}
