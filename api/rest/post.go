package apiv1

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

type post struct {
	ID          int
	Cid         string
	Sha256      string
	Md5         string
	Filesize    int64
	Mime        string
	Removed     bool
	Deleted     bool
	Description string
	Dimension   DM.Dimension
	Thumbnails  []DM.Thumb
	Metadata    map[string][]string
	Tags        []tagInterface
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	p := DM.NewPost()

	firstNonEmpty := func(keys ...string) (string, string) {
		for _, key := range keys {
			if value := r.FormValue(key); value != "" {
				return key, value
			}
		}
		return "", ""
	}

	key, val := firstNonEmpty("id", "ipfs", "sha256", "md5")

	var err error

	switch key {
	case "id":
		var id int
		id, err = strconv.Atoi(val)
		if err != nil {
			break
		}
		err = p.SetID(DM.DB, id)
	case "ipfs":
		p, err = DM.GetPostFromCID(val)
	case "sha256", "md5":
		p, err = DM.GetPostFromHash(key, val)
	default:
		apiError(w, errors.New("No Identifier"), http.StatusBadRequest)
		return
	}

	if err != nil {
		if err == sql.ErrNoRows {
			apiError(w, errors.New("Post Not Found"), http.StatusNotFound)
			return
		}

		apiError(w, err, http.StatusInternalServerError)
		return
	}

	var combineTags bool
	if len(r.FormValue("combTagNamespace")) > 0 {
		combineTags = true
	}

	tags, err := p.Tags()
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	AP, err := dmToAPIPost(p, tags, combineTags)
	if err != nil {
		log.Print(err)
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	writeJson(w, AP)
}
