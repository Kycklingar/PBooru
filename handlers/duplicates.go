package handlers

import (
	"net/http"
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
