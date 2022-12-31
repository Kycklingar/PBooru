package handlers

import (
	"net/http"
	"net/url"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/user"
)

func appleTreeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		pluckApple(w, r)
		return
	}

	var page struct {
		UserInfo UserInfo
		User     user.User
		Trees    []DM.AppleTree
		Report   DM.DupeReportStats
		Form     url.Values
		Query    string
		Offset   int
		BasePear bool
	}

	page.User, page.UserInfo = getUser(w, r)

	if page.User.ID <= 0 {
		http.Error(w, "Registered users only", http.StatusForbidden)
		return
	}

	var err error

	var limit = 25
	page.Offset, _ = strconv.Atoi(r.FormValue("offset"))
	page.BasePear = r.FormValue("base-pear") == "on"

	// Drop offset key
	r.Form.Del("offset")
	page.Form = r.Form

	page.Query = r.FormValue("tags")
	page.Trees, err = DM.GetAppleTrees(page.Query, page.BasePear, limit, page.Offset)
	if internalError(w, err) {
		return
	}

	page.Report, err = DM.GetDupeReportDelay()
	if internalError(w, err) {
		return
	}

	page.Offset += limit

	for _, tree := range page.Trees {
		tree.Apple.QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFRemoved,
		)
		for _, pear := range tree.Pears {
			pear.QMul(
				DM.DB,
				DM.PFCid,
				DM.PFMime,
				DM.PFRemoved,
				DM.PFThumbnails,
			)
		}
	}

	renderTemplate(w, "appletree", page)
}

func pluckApple(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)

	var (
		dupes = DM.Dupe{Post: DM.NewPost()}
		err   error
	)

	dupes.Post.ID, err = strconv.Atoi(r.FormValue("apple"))
	if badRequest(w, err) {
		return
	}

	pearsStr := r.Form["pears"]
	for _, pearStr := range pearsStr {
		var p = DM.NewPost()
		p.ID, err = strconv.Atoi(pearStr)
		if badRequest(w, err) {
			return
		}

		dupes.Inferior = append(dupes.Inferior, p)
	}

	if user.Flag.Delete() {
		err = DM.PluckApple(dupes)
		if internalError(w, err) {
			return
		}
	} else {
		err = DM.ReportDuplicates(dupes, user, "", DM.RNonDupe)
		if internalError(w, err) {
			return
		}
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
	return
}
