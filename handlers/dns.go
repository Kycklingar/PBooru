package handlers

import (
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func dnsHandler(w http.ResponseWriter, r *http.Request) {
	dc, err := DM.ListDnsCreators()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "dns", dc)
}

func dnsCreatorHandler(w http.ResponseWriter, r *http.Request) {
	uri := uriSplitter(r)
	id, err := uri.getIntAtIndex(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c, err := DM.GetDnsCreator(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "dns_creator", c)
}
