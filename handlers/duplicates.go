package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func dupReportsHandler(w http.ResponseWriter, r *http.Request) {
	var page = struct {
		UserInfo UserInfo
		User     *DM.User
		Reports  []*DM.DupReport
	}{}

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)


	offset, _ := strconv.Atoi(r.FormValue("offset"))
	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		limit = 10
	}

	order := r.FormValue("order") == "asc"

	page.Reports, err = DM.FetchDupReports(limit, offset, order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, report := range page.Reports {
		report.Reporter.QName(DM.DB)
		report.Dupe.Post = DM.CachedPost(report.Dupe.Post)
		report.Dupe.Post.QMul(
			DM.DB,
			DM.PFThumbnails,
			DM.PFRemoved,
		)
		for _, post := range report.Dupe.Inferior {
			post = DM.CachedPost(post)
			post.QMul(
				DM.DB,
				DM.PFThumbnails,
				DM.PFRemoved,
			)
		}
	}

	renderTemplate(w, "dup-reports", page)
}

func dupReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	var note = r.FormValue("note")
	user, _ := getUser(w, r)

	const (
		bestPostIDKey = "best-id"
	)

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

	if d.Post == nil {
		http.Error(w, "No superior post has been choosen", http.StatusBadRequest)
	}

	// Assign duplicates if having sufficient privileges
	// otherwise submit a report
	if user.QFlag(DM.DB).Delete() {
		if err = DM.AssignDuplicates(d, user); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if report := r.FormValue("report-id"); report != "" {
			processReportHandler(w, r)
			return
		}
	} else {
		if err = DM.ReportDuplicates(d, user, note); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(w, "Thank you for your report")
}

func processReportHandler(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.Atoi(r.FormValue("report-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Delete() {
		http.Error(w, lackingPermissions("Delete"), http.StatusBadRequest)
		return
	}

	err = DM.ProcessDupReport(reportID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
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

		p.QMul(
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
