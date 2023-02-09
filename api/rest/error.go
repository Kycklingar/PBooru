package apiv1

import (
	"fmt"
	"net/http"
)

func apiError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, "{\"Error\": \"%v\", \"Code\": %d}", err, code)
}
