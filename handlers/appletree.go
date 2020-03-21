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
		User *DM.User
		Trees    []DM.AppleTree
		Query string
		Offset int
	}

	page.User, page.UserInfo = getUser(w, r)
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
		tree.Apple.QThumbnails(DM.DB)
		tree.Apple.QHash(DM.DB)
		tree.Apple.QMime(DM.DB)
		tree.Apple.QDeleted(DM.DB)
		for _, pear := range tree.Pears {
			pear.QThumbnails(DM.DB)
			pear.QHash(DM.DB)
			pear.QMime(DM.DB)
			pear.QDeleted(DM.DB)
		}
	}

	renderTemplate(w, "appletree", page)
}

func pluckApple(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w, r)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Delete() {
		http.Error(w, lackingPermissions("Delete"), http.StatusBadRequest)
		return
	}

	apple, err := strconv.Atoi(r.FormValue("apple"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var pears []int

	pearsStr := r.Form["pears"]
	for _, pearStr := range pearsStr {
		pear, err := strconv.Atoi(pearStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		pears = append(pears, pear)
	}

	err = DM.PluckApple(apple, pears)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
	return
}
