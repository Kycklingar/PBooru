package handlers

import (
	"log"
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func appleTreeHandler(w http.ResponseWriter, r *http.Request) {
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
