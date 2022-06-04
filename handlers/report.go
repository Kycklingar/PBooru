package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func reportsHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)
	if !u.QFlag(DM.DB).Special() {
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

	for _, report := range p.Reports {
		report.Reporter = DM.CachedUser(report.Reporter)
		report.Reporter.QName(DM.DB)
	}

	renderTemplate(w, "reports", p)
}

func reportDeleteHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	if !u.QFlag(DM.DB).Special() {
		http.Error(w, "insufficent privileges. Want 'Special'", http.StatusBadRequest)
		return
	}

	rid, err := strconv.Atoi(r.FormValue("report-id"))
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	if u.QID(DM.DB) <= 0 {
		http.Error(w, "user not logged in", http.StatusBadRequest)
		return
	}

	postID, err := strconv.Atoi(r.FormValue("post-id"))
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var report = DM.NewReport()

	report.Reason, err = strconv.Atoi(r.FormValue("reason"))
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	report.Description = r.FormValue("description")
	report.Post.ID = postID
	report.Reporter = u

	internalError(w, report.Submit())

	fmt.Fprint(w, "Thank you for the report, an admin will soon review the post")
}
