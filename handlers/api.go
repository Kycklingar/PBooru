package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	BM "github.com/kycklingar/PBooru/benchmark"
)

func APIHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "api", nil)
}

func APIv1Handler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "apiV1", nil)
}

type APIv1Post struct {
	ID          int
	Hash        string
	Sha256      string
	Md5         string
	ThumbHashes []DM.Thumb
	Mime        string
	Removed     bool
	Deleted     bool
	Tags        []APIv1TagI
	Dimension   DM.Dimension
	Filesize    int64
}

type APIv1TagI interface {
	Parse(*DM.Tag)
}

type APIv1TagString string

func (t *APIv1TagString) Parse(tag *DM.Tag) {
	var str string
	if tag.QNamespace(DM.DB).QNamespace(DM.DB) == "none" {
		str = tag.QTag(DM.DB)
	} else {
		str = fmt.Sprintf("%s:%s", tag.QNamespace(DM.DB).QNamespace(DM.DB), tag.QTag(DM.DB))
	}

	*t = APIv1TagString(str)
}

type APIv1Tag struct {
	Tag       string
	Namespace string
	Count     int
}

func (t *APIv1Tag) Parse(tag *DM.Tag) {
	t.Tag = tag.QTag(DM.DB)
	t.Namespace = tag.QNamespace(DM.DB).QNamespace(DM.DB)
	t.Count = tag.Count
}

func jsonEncode(w http.ResponseWriter, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	enc.SetIndent("", " ")
	err := enc.Encode(v)
	if err != nil {
		log.Print(err)
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return err
	}
	return nil
}

func APIv1PostHandler(w http.ResponseWriter, r *http.Request) {
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
		APIError(w, "No Identifier", http.StatusBadRequest)
		return
	}

	if err != nil {
		if err == sql.ErrNoRows {
			APIError(w, "Post Not Found", http.StatusNotFound)
			return
		}

		APIError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var combineTags bool
	if len(r.FormValue("combTagNamespace")) > 0 {
		combineTags = true
	}

	tc := DM.TagCollector{}
	err = tc.FromPostMul(
		DM.DB,
		p,
		DM.FTag,
		DM.FCount,
		DM.FNamespace,
	)
	if err != nil {
		APIError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	AP, err := DMToAPIPost(p, tc.Tags, combineTags)
	if err != nil {
		log.Print(err)
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	jsonEncode(w, AP)
}

//func APIv1DuplicateHandler(w http.ResponseWriter, r *http.Request) {
//	p := DM.NewPost()
//	if postID := r.FormValue("id"); postID != "" {
//		id, err := strconv.Atoi(postID)
//		if err != nil {
//			APIError(w, "ID is not a number", http.StatusBadRequest)
//			return
//		}
//
//		err = p.SetID(DM.DB, id)
//		if err != nil {
//			log.Print(err)
//			APIError(w, ErrInternal, http.StatusInternalServerError)
//			return
//		}
//	} else if postHash := r.FormValue("hash"); postHash != "" {
//		p.Hash = postHash
//
//		if p.QID(DM.DB) == 0 {
//			//fmt.Fprint(w, "{}")
//			APIError(w, "Post Not Found", http.StatusNotFound)
//			return
//		}
//	} else {
//		APIError(w, "No Identifier", http.StatusBadRequest)
//		return
//	}
//
//	d, err := p.Duplicates()
//
//	type APIv1Duplicate struct {
//		ID    int
//		Level int
//	}
//
//	dp := APIv1Duplicate{d.QDupID(DM.DB), d.QLevel(DM.DB)}
//
//	jsonEncode(w, dp)
//}

func DMToAPIPost(p *DM.Post, tags []*DM.Tag, combineTagNamespace bool) (APIv1Post, error) {
	var AP APIv1Post

	if err := p.QMul(
		DM.DB,
		DM.PFHash,
		DM.PFMime,
		DM.PFRemoved,
		DM.PFDeleted,
		DM.PFSize,
		DM.PFChecksums,
		DM.PFThumbnails,
		DM.PFDimension,
	); err != nil {
		log.Println(err)
	}

	AP = APIv1Post{
		ID:          p.ID,
		Hash:        p.Hash,
		Sha256:      p.Checksums.Sha256,
		Md5:         p.Checksums.Md5,
		ThumbHashes: p.Thumbnails(),
		Mime:        p.Mime.Str(),
		Removed:     p.Removed,
		Deleted:     p.Deleted,
		Filesize:    p.Size,
		Dimension:   p.Dimension,
	}

	for _, tag := range tags {
		var t APIv1TagI
		if combineTagNamespace {
			t = new(APIv1TagString)
		} else {
			t = &APIv1Tag{}
		}
		t.Parse(tag)
		AP.Tags = append(AP.Tags, t)
	}

	return AP, nil
}

type APIv1Posts struct {
	TotalPosts int
	Generated  float64
	Posts      []APIv1Post
}

func APIv1PostsHandler(w http.ResponseWriter, r *http.Request) {
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
	err = pc.Get(tagStr, orStr, filterStr, unlessStr, order, mimeIDs, 0, groupAlts)
	if err != nil {
		log.Print(err)
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	pc = DM.CachedPostCollector(pc)

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	} else {
		limit = DM.Larg(10, DM.Smal(100, limit))
	}

	var AP APIv1Posts

	result, err := pc.Search2(limit, limit*offset)
	if err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}
	AP.Posts = make([]APIv1Post, len(result))
	for i, set := range result {
		APp, err := DMToAPIPost(set.Post, set.Tags, combineTags)
		if err != nil {
			log.Print(err)
			http.Error(w, ErrInternal, http.StatusInternalServerError)
			return
		}
		//AP.Posts = append(AP.Posts, APp)
		AP.Posts[i] = APp

	}

	AP.TotalPosts = pc.TotalPosts

	AP.Generated = bm.End(false).Seconds()
	jsonEncode(w, AP)
}

func APIError(w http.ResponseWriter, err string, code int) {
	s := fmt.Sprintf("{\"Error\": \"%s\", \"Code\": %d}", err, code)
	http.Error(w, s, code)
	return
}

func APIv1ComicsHandler(w http.ResponseWriter, r *http.Request) {
	cc := DM.ComicCollector{}
	if err := cc.Search(r.FormValue("title"), r.FormValue("tags"), 10, 0); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
	}

	// cm := tComics(5, cc.Comics)
	// cm[0].Chapters
}

func APIv1SuggestTagsHandler(w http.ResponseWriter, r *http.Request) {
	tagStr := r.FormValue("tags")
	timer := BM.Begin()
	var tc DM.TagCollector
	tc.Parse(tagStr, ",")

	if len(r.FormValue("opensearch")) >= 1 {
		jsonEncode(w, openSearchSuggestions(tagStr, tc))
	} else {
		sugt := tc.SuggestedTags(DM.DB)

		var tags []APIv1Tag
		for _, t := range sugt.Tags {
			var nt APIv1Tag
			nt.Tag = t.QTag(DM.DB)
			nt.Namespace = t.QNamespace(DM.DB).QNamespace(DM.DB)

			tags = append(tags, nt)
		}
		jsonEncode(w, tags)
	}

	timer.End(performBenchmarks)
}

func openSearchSuggestions(query string, tc DM.TagCollector) []interface{} {
	var tags []string
	//var counts []string
	for _, t := range tc.SuggestedTags(DM.DB).Tags {
		var str string
		tag := t.QTag(DM.DB)
		namespace := t.QNamespace(DM.DB).QNamespace(DM.DB)
		if namespace == "none" {
			str = tag
		} else {
			str = namespace + ":" + tag
		}

		tags = append(tags, str)
		//counts = append(counts, fmt.Sprint(t.QCount(DM.DB), " results"))
	}

	return []interface{}{query, tags} //, counts}
}

func APIv1SimilarPostsHandler(w http.ResponseWriter, r *http.Request) {
	postIDstr := r.FormValue("id")
	postID, err := strconv.Atoi(postIDstr)
	if err != nil {
		APIError(w, "valid id required", http.StatusBadRequest)
		return
	}

	var combineTags bool
	if len(r.FormValue("combTagNamespace")) > 0 {
		combineTags = true
	}

	type page struct {
		Posts []APIv1Post
	}

	var p page

	post := DM.NewPost()
	if err = post.SetID(DM.DB, postID); err != nil {
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}
	var posts []*DM.Post
	if posts, err = post.FindSimilar(DM.DB, 5, false); err != nil {
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	for _, pst := range posts {
		var ps APIv1Post
		if ps, err = DMToAPIPost(pst, nil, combineTags); err != nil {
			APIError(w, ErrInternal, http.StatusInternalServerError)
			return
		}
		p.Posts = append(p.Posts, ps)
	}
	jsonEncode(w, p)
}
