package handlers

import (
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func archiveHandler(w http.ResponseWriter, r *http.Request) {
	u := uriSplitter(r)
	id, err := u.getAtIndex(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a := DM.Archive(id)

	var p = struct {
		Cid   string
		State DM.ProgressState
	}{
		a.Cid,
		a.Progress(),
	}

	renderTemplate(w, "archive", p)
}

func createArchiveHandler(w http.ResponseWriter, r *http.Request) {
	tags := r.FormValue("tags")
	or := r.FormValue("or")
	filter := r.FormValue("filter")
	unless := r.FormValue("unless")

	altgroup, _ := strconv.Atoi(r.FormValue("alt-group"))

	mimeGroups := r.Form["mime-type"]

	mimeIDs := DM.MimeIDsFromType(mimeGroups)

	mimes := r.Form["mime"]
	for _, mime := range mimes {
		id, err := strconv.Atoi(mime)
		if err != nil {
			log.Println(err)
			continue
		}
		contains := func(s []int, i int) bool {
			for _, x := range s {
				if x == i {
					return true
				}
			}

			return false
		}

		if !contains(mimeIDs, id) {
			mimeIDs = append(mimeIDs, id)
		}
	}

	pc := DM.NewPostCollector()
	err := pc.Get(
		tags, or,
		filter, unless,
		"ASC",
		mimeIDs,
		altgroup,
		false,
	)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a, err := pc.ArchiveSearch()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/archive/"+a.ID, http.StatusSeeOther)

}
