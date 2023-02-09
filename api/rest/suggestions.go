package apiv1

import (
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func suggestTagsHandler(w http.ResponseWriter, r *http.Request) {
	tagStr := r.FormValue("tags")

	hints, err := DM.TagHints(tagStr)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	if len(r.FormValue("opensearch")) >= 1 {
		writeJson(w, openSearchSuggestions(tagStr, hints))
	} else {
		var tags = make([]tag, len(hints))
		for i := range hints {
			tags[i].Tag = hints[i].Tag
			tags[i].Namespace = string(hints[i].Namespace)
			tags[i].Count = hints[i].Count
		}
		if len(tags) > 0 {
			writeJson(w, tags)
		} else {
			writeJson(w, nil)
		}
	}
}

func openSearchSuggestions(query string, hints []DM.Tag) []interface{} {
	var tags []string
	for _, t := range hints {
		tags = append(tags, t.String())
	}

	return []interface{}{query, tags} //, counts}
}
