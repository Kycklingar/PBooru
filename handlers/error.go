package handlers

import (
	"fmt"
	"net/http"
)

func permError(w http.ResponseWriter, perm string) {
	badRequest(w, fmt.Errorf("You are not allowed to do that, peasant.\nMissing privilege: %s", perm))
}

func badRequest(w http.ResponseWriter, err error) bool {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return true
	}
	return false
}

func internalError(w http.ResponseWriter, err error) bool {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}
