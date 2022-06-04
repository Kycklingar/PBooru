package handlers

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
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
		_, fn, line, _ := runtime.Caller(1)
		log.Printf("\n\t%s:%d:\n\t\t%v\n", fn, line, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}
