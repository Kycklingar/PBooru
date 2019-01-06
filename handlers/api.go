package handlers

import (
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
	ID        int
	Hash      string
	ThumbHash string
	Mime      string
	Deleted   bool
	Tags      []APIv1Tag
}

type APIv1Tag struct {
	Tag       string
	Namespace string
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

	if postID := r.FormValue("id"); postID != "" {
		id, err := strconv.Atoi(postID)
		if err != nil {
			APIError(w, "ID is not a number", http.StatusBadRequest)
			return
		}

		err = p.SetID(DM.DB, id)
		if err != nil {
			log.Print(err)
			APIError(w, ErrInternal, http.StatusInternalServerError)
			return
		}
	} else if postHash := r.FormValue("hash"); postHash != "" {
		p.Hash = postHash

		if p.QID(DM.DB) == 0 {
			APIError(w, "Post Not Found", http.StatusNotFound)
			return
		}
	} else {
		APIError(w, "No Identifier", http.StatusBadRequest)
		return
	}

	AP, err := DMToAPIPost(p)
	if err != nil {
		log.Print(err)
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	jsonEncode(w, AP)
}

func APIv1DuplicateHandler(w http.ResponseWriter, r *http.Request) {
	p := DM.NewPost()
	if postID := r.FormValue("id"); postID != "" {
		id, err := strconv.Atoi(postID)
		if err != nil {
			APIError(w, "ID is not a number", http.StatusBadRequest)
			return
		}

		err = p.SetID(DM.DB, id)
		if err != nil {
			log.Print(err)
			APIError(w, ErrInternal, http.StatusInternalServerError)
			return
		}
	} else if postHash := r.FormValue("hash"); postHash != "" {
		p.Hash = postHash

		if p.QID(DM.DB) == 0 {
			//fmt.Fprint(w, "{}")
			APIError(w, "Post Not Found", http.StatusNotFound)
			return
		}
	} else {
		APIError(w, "No Identifier", http.StatusBadRequest)
		return
	}

	d := DM.NewDuplicate()
	d.Post = p

	type APIv1Duplicate struct {
		ID    int
		Level int
	}

	dp := APIv1Duplicate{d.QDupID(DM.DB), d.QLevel(DM.DB)}

	jsonEncode(w, dp)
}

func DMToAPIPost(p *DM.Post) (APIv1Post, error) {
	var AP APIv1Post
	tc := DM.TagCollector{}
	// err := tc.GetFromPost(DM.DB, *p)
	err := tc.GetPostTags(DM.DB, p)
	if err != nil {
		return AP, err
	}

	AP = APIv1Post{ID: p.QID(DM.DB), Hash: p.QHash(DM.DB), ThumbHash: p.QThumb(DM.DB), Mime: p.QMime(DM.DB).Str(), Deleted: p.QDeleted(DM.DB) == 1}

	for _, tag := range tc.Tags {
		AP.Tags = append(AP.Tags, APIv1Tag{tag.QTag(DM.DB), tag.QNamespace(DM.DB).QNamespace(DM.DB)})
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
	filterStr := r.FormValue("filter")
	unlessStr := r.FormValue("unless")
	order := r.FormValue("order")
	offsetStr := r.FormValue("offset")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	bm := BM.Begin()

	pc := &DM.PostCollector{}
	err = pc.Get(tagStr, filterStr, unlessStr, order)
	if err != nil {
		log.Print(err)
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	DM.CachedPostCollector(pc)

	var AP APIv1Posts

	AP.TotalPosts = pc.TotalPosts

	for _, post := range pc.Search(30, 30*offset) {
		APp, err := DMToAPIPost(post)
		if err != nil {
			log.Print(err)
			http.Error(w, ErrInternal, http.StatusInternalServerError)
			return
		}
		AP.Posts = append(AP.Posts, APp)
	}

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
	if err := cc.Get(10, 0); err != nil {
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
	tc.Parse(tagStr)

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
	if posts, err = post.FindSimilar(DM.DB, 5); err != nil {
		APIError(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	for _, pst := range posts {
		var ps APIv1Post
		if ps, err = DMToAPIPost(pst); err != nil {
			APIError(w, ErrInternal, http.StatusInternalServerError)
			return
		}
		p.Posts = append(p.Posts, ps)
	}
	jsonEncode(w, p)
}
