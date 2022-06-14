package handlers

import (
	"net/http"
	"strconv"
	"strings"

	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"
)

func searchLogsHandler(w http.ResponseWriter, r *http.Request) {
	uri := uriSplitter(r)
	var (
		opts DM.LogSearchOptions
		err  error
		cat  string
	)

	if uri.length() > 2 {
		cat, err = uri.getAtIndex(1)
		if badRequest(w, err) {
			return
		}

		opts.CatVal, err = uri.getIntAtIndex(2)
		if badRequest(w, err) {
			return
		}
	}

	switch strings.ToLower(cat) {
	case "post":
		opts.Category = DM.LogCatPost
	case "comic":
		opts.Category = DM.LogCatComic
	case "chapter":
		opts.Category = DM.LogCatChapter
	}

	renderSpine(w, r, opts)
}

func renderSpine(w http.ResponseWriter, r *http.Request, opts DM.LogSearchOptions) {
	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil {
		page = 1
	}

	page = mm.Max(1, page)

	opts.Limit = 50
	opts.Offset = opts.Limit * (page - 1)
	logs, count, err := DM.SearchLogs(opts)
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
		Logs         []DM.Log
		UserInfo     UserInfo
		NextPage     int
		PreviousPage int
		Showing      int
		To           int
		OutOf        int
	}{
		Logs:         logs,
		UserInfo:     ui,
		NextPage:     page + 1,
		PreviousPage: page - 1,
		Showing:      opts.Offset + 1,
		To:           mm.Min(count, opts.Offset+opts.Limit),
		OutOf:        count,
	}

	if opts.Offset+opts.Limit > count {
		p.NextPage = 0
	}

	renderTemplate(w, "logs", p)
}
