package apiv1

import (
	"encoding/json"
	"log"
	"net/http"
)

func writeJson(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)

	enc.SetEscapeHTML(true)
	enc.SetIndent("", "\t")

	err := enc.Encode(v)
	if err != nil {
		log.Print(err)
		apiError(w, err, http.StatusInternalServerError)
		return err
	}
	return nil
}
