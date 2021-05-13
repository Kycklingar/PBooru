package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func tombstoneHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		limit = 30
		page  struct {
			UserInfo  UserInfo
			Tombstone []DM.Tombstone
			Paginator paginator
			Total     int
		}
	)

	_, page.UserInfo = getUser(w, r)

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

	page.Tombstone, err = DM.GetTombstonedPosts(limit, (currentPage-1)*limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page.Total = DM.Tombstones

	for _, p := range page.Tombstone {
		if err = p.Post.QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFMime,
			DM.PFRemoved,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	page.Paginator = paginator{
		current: currentPage,
		last:    DM.Tombstones / limit,
		plength: 30,
		format:  fmt.Sprintf("/tombstone/%s/", "%d"),
	}

	renderTemplate(w, "tombstone", page)

}
