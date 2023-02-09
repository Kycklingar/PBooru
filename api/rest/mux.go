package apiv1

import (
	"net/http"

	"github.com/kycklingar/PBooru/handlers"
	"github.com/kycklingar/PBooru/middleware"
)

var Mux http.Handler

func init() {
	var mux = http.NewServeMux()
	m := mux.HandleFunc
	m("/", handlers.APIv1Handler)
	m("/post", postHandler)
	m("/posts", postsHandler)
	m("/similar", similarPostsHandler)
	m("/suggestions", suggestTagsHandler)

	Mux = middleware.Cors(mux)
}
