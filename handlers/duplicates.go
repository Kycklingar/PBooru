package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func dupReportsHandler(w http.ResponseWriter, r *http.Request) {
	var page = struct {
		UserInfo UserInfo
		User     *DM.User
		Reports  []*DM.DupReport
		Form     url.Values
	}{}

	page.User, page.UserInfo = getUser(w, r)
	page.User.QFlag(DM.DB)

	offset, _ := strconv.Atoi(r.FormValue("offset"))
	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		limit = 10
	}

	page.Form = r.Form

	plucked := r.FormValue("plucked") == "on"

	order := r.FormValue("order") == "asc"

	approved := r.FormValue("approved") == "on"

	page.Reports, err = DM.FetchDupReports(limit, offset, order, approved, plucked)
	if internalError(w, err) {
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

	var (
		note           = r.FormValue("note")
		reportNonDupes = r.FormValue("non-dupes") == "on"
	)

	user, _ := getUser(w, r)

	const (
		bestPostIDKey = "best-id"
	)

	m, err := verifyInteger(r, bestPostIDKey)
	if badRequest(w, err) {
		return
	}

	var (
		dupes   DM.Dupe
		removed DM.Dupe
	)

	for _, idstr := range r.Form["post-ids"] {
		p := DM.NewPost()

		p.ID, err = strconv.Atoi(idstr)
		if badRequest(w, err) {
			return
		}

		if p.ID == m[bestPostIDKey] {
			dupes.Post = p
			removed.Post = p
		} else {
			dupes.Inferior = append(dupes.Inferior, p)
		}
	}

	for _, idstr := range r.Form["removed-ids"] {
		p := DM.NewPost()

		p.ID, err = strconv.Atoi(idstr)
		if badRequest(w, err) {
			return
		}

		removed.Inferior = append(removed.Inferior, p)
	}

	if dupes.Post == nil {
		http.Error(w, "No superior post has been choosen", http.StatusBadRequest)
		return
	}

	// Assign duplicates if having sufficient privileges
	// otherwise submit a report
	if user.QFlag(DM.DB).Delete() {
		if internalError(w, DM.AssignDuplicates(dupes, user)) {
			return
		}

		if reportNonDupes && len(removed.Inferior) >= 1 {
			if internalError(w, DM.PluckApple(removed)) {
				return
			}
		}

		if report := r.FormValue("report-id"); report != "" {
			processReportHandler(w, r)
			return
		}
	} else {
		if internalError(w, DM.ReportDuplicates(dupes, user, note, DM.RDupe)) {
			return
		}

		if reportNonDupes && len(removed.Inferior) >= 1 {
			if internalError(w, DM.ReportDuplicates(removed, user, note, DM.RNonDupe)) {
				return
			}
		}
	}

	fmt.Fprint(w, "Thank you for your report")
}

func processReportHandler(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.Atoi(r.FormValue("report-id"))
	if badRequest(w, err) {
		return
	}

	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Delete() {
		http.Error(w, lackingPermissions("Delete"), http.StatusBadRequest)
		return
	}

	err = DM.ProcessDupReport(reportID)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func processPluckReportHandler(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.Atoi(r.FormValue("report-id"))
	if badRequest(w, err) {
		return
	}

	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Delete() {
		http.Error(w, lackingPermissions("Delete"), http.StatusBadRequest)
		return
	}

	rep, err := DM.FetchDupReport(reportID, DM.DB)
	if internalError(w, err) {
		return
	}

	err = DM.PluckApple(rep.Dupe)
	if internalError(w, err) {
		return
	}

	err = DM.ProcessDupReport(reportID)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func compareReportHandler(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.Atoi(r.FormValue("report-id"))
	if badRequest(w, err) {
		return
	}

	report, err := DM.FetchDupReport(reportID, DM.DB)
	if badRequest(w, err) {
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
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFDimension,
			DM.PFSize,
			DM.PFRemoved,
		)
	}

	renderTemplate(w, "compare2", page)
}

func dupReportCleanupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	aff, err := DM.DuplicateReportCleanup()
	if internalError(w, err) {
		return
	}

	fmt.Fprint(w, aff, " reports affected")
}
