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
	if internalError(w, err) {
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
			if internalError(w, err) {
				return
			}
		}

		for _, a := range log.Alts {
			for _, p := range a.Posts {
				err := p.QMul(
					DM.DB,
					DM.PFHash,
					DM.PFMime,
					DM.PFThumbnails,
				)
				if internalError(w, err) {
					return
				}
			}
		}

		for _, a := range log.Aliases {
			for _, t := range append(a.From, a.To) {
				err := t.QueryAll(DM.DB)
				if internalError(w, err) {
					return
				}
			}
		}

		for _, t := range append(log.Parents.Parents, log.Parents.Children...) {
			err := t.QueryAll(DM.DB)
			if err != nil {
				internalError(w, err)
				return
			}
		}

		for _, mls := range log.MultiTags {
			for _, ml := range mls {
				err := ml.Tag.QueryAll(DM.DB)
				if internalError(w, err) {
					return
				}
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
