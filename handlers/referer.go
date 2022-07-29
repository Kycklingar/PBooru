package handlers

import (
	"fmt"
	"net/http"
)

func toRefOrOK(w http.ResponseWriter, r *http.Request, message string) {
	if r.Referer() != "" {
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	fmt.Fprint(w, message)
}

func toRefOrBackup(w http.ResponseWriter, r *http.Request, backup string) {
	s := r.Referer()
	if len(s) <= 0 {
		s = backup
	}

	http.Redirect(w, r, s, http.StatusSeeOther)
}
