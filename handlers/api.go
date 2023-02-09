package handlers

import (
	"net/http"
)

func APIHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "api", nil)
}

func APIv1Handler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "apiV1", nil)
}
