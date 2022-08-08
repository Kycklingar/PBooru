package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"
)

var logCat = map[string]DM.LogCategory{
	"post":      DM.LogCatPost,
	"comic":     DM.LogCatComic,
	"chapter":   DM.LogCatChapter,
	"comicpage": DM.LogCatComicPage,
}

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

	opts.Category = logCat[strings.ToLower(cat)]

	renderSpine(w, r, opts)
}

func parseTime(layout, value string) (time.Time, error) {
	max := mm.Min(len(layout), len(value))
	return time.ParseInLocation(layout[:max], value[:max], time.Local)
}

func renderSpine(w http.ResponseWriter, r *http.Request, opts DM.LogSearchOptions) {
	var err error
	r.ParseForm()

	if user, ok := r.Form["user"]; ok {
		opts.UserID, err = strconv.Atoi(user[0])
		if badRequest(w, err) {
			return
		}
	}

	const layout = "2006-01-02 15:04"

	if dateSince := r.FormValue("date-since"); dateSince != "" {
		since := fmt.Sprint(dateSince, " ", r.FormValue("time-since"))
		opts.DateSince, err = parseTime(layout, since)
		if badRequest(w, err) {
			return
		}
	}

	if dateUntil := r.FormValue("date-until"); dateUntil != "" {
		until := fmt.Sprint(dateUntil, " ", r.FormValue("time-until"))
		opts.DateUntil, err = parseTime(layout, until)
		if badRequest(w, err) {
			return
		}
	}

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
				DM.PFCid,
				DM.PFMime,
				DM.PFThumbnails,
				DM.PFRemoved,
			)
			if internalError(w, err) {
				return
			}

			for i := range ph.Duplicates.Inferior {
				err = ph.Duplicates.Inferior[i].QMul(
					DM.DB,
					DM.PFCid,
					DM.PFMime,
					DM.PFThumbnails,
					DM.PFRemoved,
				)
				if internalError(w, err) {
					return
				}
			}
		}

		for _, a := range log.Alts {
			for _, p := range a.Posts {
				err := p.QMul(
					DM.DB,
					DM.PFCid,
					DM.PFMime,
					DM.PFThumbnails,
					DM.PFRemoved,
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

		for _, page := range log.ComicPages {
			err = page.Post.QMul(
				DM.DB,
				DM.PFCid,
				DM.PFMime,
				DM.PFThumbnails,
				DM.PFRemoved,
			)
			if internalError(w, err) {
				return
			}

			if page.Diff != nil {
				err = page.Diff.Post.QMul(
					DM.DB,
					DM.PFCid,
					DM.PFMime,
					DM.PFThumbnails,
					DM.PFRemoved,
				)
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
		Form         url.Values
	}{
		Logs:         logs,
		UserInfo:     ui,
		NextPage:     page + 1,
		PreviousPage: page - 1,
		Showing:      mm.Min(count, opts.Offset+1),
		To:           mm.Min(count, opts.Offset+opts.Limit),
		OutOf:        count,
		Form:         r.Form,
	}

	if opts.Offset+opts.Limit > count {
		p.NextPage = 0
	}

	renderTemplate(w, "logs", p)
}
