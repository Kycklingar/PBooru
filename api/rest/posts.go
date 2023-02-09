package apiv1

import (
	"log"
	"net/http"
	"strconv"

	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"
	BM "github.com/kycklingar/PBooru/benchmark"
)

type posts struct {
	TotalPosts int
	Generated  float64
	Posts      []post
}

func postsHandler(w http.ResponseWriter, r *http.Request) {
	tagStr := r.FormValue("tags")
	orStr := r.FormValue("or")
	filterStr := r.FormValue("filter")
	unlessStr := r.FormValue("unless")
	limitStr := r.FormValue("limit")
	order := r.FormValue("order")
	offsetStr := r.FormValue("offset")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	mimeGroups := r.Form["mime-type"]

	mimeIDs := DM.MimeIDsFromType(mimeGroups)

	mimes := r.Form["mime"]
	for _, mime := range mimes {
		id, err := strconv.Atoi(mime)
		if err != nil {
			log.Println(err)
			continue
		}
		contains := func(s []int, i int) bool {
			for _, x := range s {
				if x == i {
					return true
				}
			}

			return false
		}

		if !contains(mimeIDs, id) {
			mimeIDs = append(mimeIDs, id)
		}
	}

	var combineTags = len(r.FormValue("combTagNamespace")) > 0
	var groupAlts = len(r.FormValue("alts")) > 0

	bm := BM.Begin()

	pc := DM.NewPostCollector()
	err = pc.Get(
		DM.SearchOptions{
			And:        tagStr,
			Or:         orStr,
			Filter:     filterStr,
			Unless:     unlessStr,
			MimeIDs:    mimeIDs,
			AltCollect: groupAlts,
			Order:      order,
		},
	)
	if err != nil {
		log.Print(err)
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	pc = DM.CachedPostCollector(pc)

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	} else {
		limit = mm.Max(10, mm.Min(100, limit))
	}

	var AP posts

	result, err := pc.Search2(limit, limit*offset)
	if err != nil {
		log.Println(err)
		http.Error(w, "FIXME", http.StatusInternalServerError)
		return
	}
	AP.Posts = make([]post, len(result))
	for i, set := range result {
		APp, err := dmToAPIPost(set.Post, set.Tags, combineTags)
		if err != nil {
			log.Print(err)
			http.Error(w, "FIXME", http.StatusInternalServerError)
			return
		}
		//AP.Posts = append(AP.Posts, APp)
		AP.Posts[i] = APp

	}

	AP.TotalPosts = pc.TotalPosts

	AP.Generated = bm.End(false).Seconds()
	writeJson(w, AP)
}
