package handlers

import (
	"log"
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func appleTreeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		pluckApple(w, r)
		return
	}

	var page struct {
		UserInfo UserInfo
		Fruits []DM.Fruit
	}

	page.UserInfo = userCookies(w, r)

	var err error

	page.Fruits, err = DM.GetFruits()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, fruit := range page.Fruits{
		fruit.Apple.QThumbnails(DM.DB)
		fruit.Apple.QHash(DM.DB)
		fruit.Apple.QMime(DM.DB)
		fruit.Apple.QDeleted(DM.DB)
		fruit.Pear.QThumbnails(DM.DB)
		fruit.Pear.QHash(DM.DB)
		fruit.Pear.QMime(DM.DB)
		fruit.Pear.QDeleted(DM.DB)

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

	const (
		appleKey = "apple"
		pearKey = "pear"
	)

	m, err := verifyInteger(r, appleKey, pearKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = DM.PluckApple(m[appleKey], m[pearKey])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
	return
}
