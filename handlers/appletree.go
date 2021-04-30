package handlers

import (
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func appleTreeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		pluckApple(w, r)
		return
	}

	var page struct {
		UserInfo UserInfo
		User     *DM.User
		Trees    []DM.AppleTree
		Query    string
		Offset   int
	}

	page.User, page.UserInfo = getUser(w, r)

	if page.User.QID(DM.DB) <= 0 {
		http.Error(w, "Registered users only", http.StatusForbidden)
		return
	}

	page.User.QFlag(DM.DB)

	var err error

	var limit = 25
	page.Offset, _ = strconv.Atoi(r.FormValue("offset"))

	page.Query = r.FormValue("tags")
	page.Trees, err = DM.GetAppleTrees(page.Query, limit, page.Offset)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page.Offset += limit

	for _, tree := range page.Trees {
		tree.Apple.QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFRemoved,
		)
		for _, pear := range tree.Pears {
			pear.QMul(
				DM.DB,
				DM.PFHash,
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pearsStr := r.Form["pears"]
	for _, pearStr := range pearsStr {
		var p = DM.NewPost()
		p.ID, err = strconv.Atoi(pearStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		dupes.Inferior = append(dupes.Inferior, p)
	}

	if user.QFlag(DM.DB).Delete() {
		err = DM.PluckApple(dupes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		err = DM.ReportDuplicates(dupes, user, "", DM.RNonDupe)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
	return
}

