package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func reportsHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)
	if !u.Flag.Special() {
		http.Error(w, "insufficent privileges. Want 'Special'", http.StatusBadRequest)
		return
	}

	type page struct {
		Reports []*DM.Report
	}

	var p page
	var err error

	p.Reports, err = DM.GetReports(DM.DB)
	if internalError(w, err) {
		return
	}

	renderTemplate(w, "reports", p)
}

func reportDeleteHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	if !u.Flag.Special() {
		http.Error(w, "insufficent privileges. Want 'Special'", http.StatusBadRequest)
		return
	}

	rid, err := strconv.Atoi(r.FormValue("report-id"))
	if badRequest(w, err) {
		return
	}

	var rep = DM.NewReport()
	rep.ID = rid

	err = rep.Delete(DM.DB)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func reportPostHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	if u.ID <= 0 {
		http.Error(w, "user not logged in", http.StatusBadRequest)
		return
	}

	postID, err := strconv.Atoi(r.FormValue("post-id"))
	if badRequest(w, err) {
		return
	}

	var report = DM.NewReport()

	report.Reason, err = strconv.Atoi(r.FormValue("reason"))
	if badRequest(w, err) {
		return
	}

	report.Description = r.FormValue("description")
	report.Post.ID = postID
	report.Reporter = u

	internalError(w, report.Submit())

	fmt.Fprint(w, "Thank you for the report, an admin will soon review the post")
}
