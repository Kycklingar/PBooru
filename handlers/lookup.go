package handlers

import (
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

type lookupPage struct {
	Base     base
	Posts    []*DM.Post
	UserInfo UserInfo
}

func imageLookupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 51<<20)
		r.ParseMultipartForm(50 << 20)
		file, _, err := r.FormFile("file")
		if badRequest(w, err) {
			return
		}
		defer file.Close()

		dist, err := strconv.Atoi(r.FormValue("distance"))
		if badRequest(w, err) {
			return
		}

		posts, err := DM.ImageLookup(file, dist)
		if internalError(w, err) {
			return
		}

		var p lookupPage

		for _, pst := range posts {
			pst.QMul(
				DM.DB,
				DM.PFHash,
				DM.PFThumbnails,
				DM.PFRemoved,
				DM.PFMime,
			)

			p.Posts = append(p.Posts, pst)
		}

		p.UserInfo = userCookies(w, r)

		renderTemplate(w, "lookup", p)
	} else {
		renderTemplate(w, "lookup", nil)
	}
}
