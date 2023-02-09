package apiv1

import (
	"log"

	DM "github.com/kycklingar/PBooru/DataManager"
)

//func APIHandler(w http.ResponseWriter, r *http.Request) {
//	renderTemplate(w, "api", nil)
//}
//
//func APIv1Handler(w http.ResponseWriter, r *http.Request) {
//	renderTemplate(w, "apiV1", nil)
//}

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
//		if p.ID == 0 {
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

func dmToAPIPost(p *DM.Post, tags []DM.Tag, combineTagNamespace bool) (post, error) {
	var AP post

	if err := p.QMul(
		DM.DB,
		DM.PFCid,
		DM.PFMime,
		DM.PFRemoved,
		DM.PFDeleted,
		DM.PFSize,
		DM.PFChecksums,
		DM.PFThumbnails,
		DM.PFDimension,
		DM.PFMetaData,
		DM.PFDescription,
	); err != nil {
		log.Println(err)
	}

	AP = post{
		ID:          p.ID,
		Cid:         p.Cid,
		Sha256:      p.Checksums.Sha256,
		Md5:         p.Checksums.Md5,
		Thumbnails:  p.Thumbnails(),
		Mime:        p.Mime.Str(),
		Removed:     p.Removed,
		Deleted:     p.Deleted,
		Filesize:    p.Size,
		Dimension:   p.Dimension,
		Description: p.Description,
	}

	AP.Metadata = make(map[string][]string)

	for namespace, datas := range p.MetaData {
		for _, data := range datas {
			AP.Metadata[namespace] = append(AP.Metadata[namespace], data.Data())
		}
	}

	for _, t := range tags {
		var ti tagInterface
		if combineTagNamespace {
			ti = new(tagString)
		} else {
			ti = new(tag)
		}
		ti.Parse(t)
		AP.Tags = append(AP.Tags, ti)
	}

	return AP, nil
}

//func APIv1ComicsHandler(w http.ResponseWriter, r *http.Request) {
//	cc := DM.ComicCollector{}
//	if err := cc.Search(r.FormValue("title"), r.FormValue("tags"), 10, 0); err != nil {
//		log.Println(err)
//		http.Error(w, ErrInternal, http.StatusInternalServerError)
//	}
//
//	// cm := tComics(5, cc.Comics)
//	// cm[0].Chapters
//}
