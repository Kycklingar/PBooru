package handlers

import (
	"net/http"
	"log"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)



func imageLookupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 51<<20)
		r.ParseMultipartForm(50 << 20)
		file, _, err := r.FormFile("file")
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		dist, err := strconv.Atoi(r.FormValue("distance"))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		posts, err := DM.ImageLookup(file, dist)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var p PostsPage

		for _, pst := range posts {
			pst.QID(DM.DB)
			pst.QHash(DM.DB)
			pst.QThumbnails(DM.DB)
			pst.QDeleted(DM.DB)
			pst.QMime(DM.DB).QName(DM.DB)
			pst.QMime(DM.DB).QType(DM.DB)

			p.Posts = append(p.Posts, pst)
		}

		p.User = userCookies(w, r)

		renderTemplate(w, "posts", p)
	} else {
		renderTemplate(w, "lookup", nil)
	}
}
