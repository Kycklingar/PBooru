package handlers

import (
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func dupReportsHandler(w http.ResponseWriter, r *http.Request) {
	var page = struct {
		UserInfo UserInfo
		Reports []*DM.DupReport
	}{}

	page.UserInfo = userCookies(w, r)

	var err error
	page.Reports, err = DM.FetchDupReports(10, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, report := range page.Reports {
		report.Reporter.QName(DM.DB)
		report.Dupe.Post = DM.CachedPost(report.Dupe.Post)
		report.Dupe.Post.QThumbnails(DM.DB)
		report.Dupe.Post.QDeleted(DM.DB)
		for _, post := range report.Dupe.Inferior {
			post = DM.CachedPost(post)
			post.QThumbnails(DM.DB)
			post.QDeleted(DM.DB)
		}
	}

	renderTemplate(w, "dup-reports", page)
}

func dupReportHandler(w http.ResponseWriter, r *http.Request) {
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
		if err = DM.ReportDuplicates(d, user, note); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func compareReportHandler(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.Atoi(r.FormValue("report-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	report, err := DM.FetchDupReport(reportID, DM.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var page comparisonPage
	page.Report = reportID
	page.UserInfo = userCookies(w, r)
	page.Posts = append([]*DM.Post{report.Dupe.Post}, report.Dupe.Inferior...)

	for _, p := range page.Posts {
		p = DM.CachedPost(p)

		p.QHash(DM.DB)
		p.QThumbnails(DM.DB)
		p.QMime(DM.DB).QName(DM.DB)
		p.QMime(DM.DB).QType(DM.DB)

		p.QDimensions(DM.DB)
		p.QSize(DM.DB)
	}

	renderTemplate(w, "compare2", page)
}
