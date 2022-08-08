package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	DM "github.com/kycklingar/PBooru/DataManager"
	paginate "github.com/kycklingar/PBooru/handlers/paginator"
)

func tombstoneSearchHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	http.Redirect(w, r, "/tombstone/?"+r.Form.Encode(), http.StatusSeeOther)
}

func tombstoneHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		limit = 30
		page  struct {
			UserInfo  UserInfo
			Tombstone []DM.Tombstone
			Paginator paginate.Paginator
			Total     int
		}
	)

	_, page.UserInfo = getUser(w, r)

	var query = r.FormValue("reason")

	var currentPage int

	uri := splitURI(r.URL.Path)
	if len(uri) <= 1 {
		currentPage = 1
	} else {
		currentPage, err = strconv.Atoi(uri[1])
		if err != nil {
			currentPage = 1
		}
	}

	page.Total, page.Tombstone, err = DM.GetTombstonedPosts(query, limit, (currentPage-1)*limit)
	if internalError(w, err) {
		return
	}

	for _, p := range page.Tombstone {
		err := p.Post.QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFRemoved,
		)
		if internalError(w, err) {
			return
		}
	}

	var q = r.Form.Encode()
	if q != "" {
		q = "?" + q
	}

	page.Paginator = paginate.New(
		currentPage,
		page.Total,
		limit,
		30,
		fmt.Sprintf("/tombstone/%%d/%s", strings.ReplaceAll(q, "%", "%%")),
	)

	renderTemplate(w, "tombstone", page)

}
