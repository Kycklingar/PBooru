package handlers

import (
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
	root := DM.Root()
	if root == "" {
		http.Error(w, "This instance does not support root", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "root", root)
}
