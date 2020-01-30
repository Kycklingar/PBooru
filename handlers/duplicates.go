package handlers

import (
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func dupReportsHandler(w http.ResponseWriter, r *http.Request) {
	reports, err := DM.FetchDupReports(10, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, report := range reports {
		report.Reporter.QName(DM.DB)
		for _, post := range report.Posts {
			post.Post.QThumbnails(DM.DB)
		}
	}

	renderTemplate(w, "dup-reports", reports)
}

func dupReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	//var note = r.FormValue("note")
	user, _ := getUser(w, r)

	const bestPostIDKey = "best-id"

	m, err := verifyInteger(r, bestPostIDKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var d DM.Dupe

	for _, idstr := range r.Form["post-ids"] {
		p := DM.NewPost()

		p.ID, err = strconv.Atoi(idstr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if p.ID == m[bestPostIDKey] {
			d.Post = p
		} else {
			d.Inferior = append(d.Inferior, p)
		}
	}

	// Assign duplicates if having sufficient privileges
	// otherwise submit a report
	if user.QFlag(DM.DB).Delete() {
		if err = DM.AssignDuplicates(d, user); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		//if err = DM.ReportDuplicates(drposts, user, note); err != nil {
		//	log.Println(err)
		//	http.Error(w, err.Error(), http.StatusInternalServerError)
		//	return
		//}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
