package handlers

import (
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func logsHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := DM.RecentLogs(DM.DB)
	renderSpine(w, r, logs, err)
}

func postLogsHandler(w http.ResponseWriter, r *http.Request) {
	uri := uriSplitter(r)
	postID, err := uri.getIntAtIndex(2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logs, err := DM.PostLogs(DM.DB, postID)
	renderSpine(w, r, logs, err)
}

func renderSpine(w http.ResponseWriter, r *http.Request, logs []DM.Log, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, ui := getUser(w, r)

	for i, log := range logs {
		logs[i].User = DM.CachedUser(log.User)
		logs[i].User.QName(DM.DB)
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

		for _, p := range log.Alts.Posts {
			err := p.QMul(
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
