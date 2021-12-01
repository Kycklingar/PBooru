package handlers

import (
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func logsHandler(w http.ResponseWriter, r *http.Request) {
	_, ui := getUser(w, r)

	logs, err := DM.RecentLogs(DM.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, log := range logs {
		for _, ph := range log.Posts {
			err := ph.Post.QMul(
				DM.DB,
				DM.PFHash,
				DM.PFMime,
				DM.PFThumbnails,
			)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	p := struct {
		Logs     []DM.Log
		UserInfo UserInfo
	}{
		Logs:     logs,
		UserInfo: ui,
	}

	renderTemplate(w, "logs", p)
}
