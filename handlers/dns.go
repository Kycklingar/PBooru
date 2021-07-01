package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	dns "github.com/kycklingar/PBooru/DataManager/dns"
)

func dnsHandler(w http.ResponseWriter, r *http.Request) {
	_, ui := getUser(w, r)

	dc, err := DM.ListDnsCreators()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var p = struct {
		UserInfo UserInfo
		Creators []DM.DnsCreator
	}{
		UserInfo: ui,
		Creators: dc,
	}

	renderTemplate(w, "dns", p)
}

func dnsCreatorHandler(w http.ResponseWriter, r *http.Request) {
	var p struct {
		UserInfo UserInfo
		Creator  DM.DnsCreator
		Domains  []string
	}

	_, p.UserInfo = getUser(w, r)

	uri := uriSplitter(r)
	id, err := uri.getIntAtIndex(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.Creator, err = DM.GetDnsCreator(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.Domains, err = DM.DnsDomains()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "dns_creator", p)
}

func dnsNewBanner(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bannerType := r.FormValue("banner-type")
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = DM.DnsNewBanner(file, creatorID, bannerType); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsAddUrl(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url := r.FormValue("url")
	domain := r.FormValue("domain")

	if err = dns.AddUrl(DM.DB, creatorID, url, domain); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsRemoveUrl(w http.ResponseWriter, r *http.Request) {
	user, _ := getUser(w, r)

	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url := r.FormValue("url")

	err = dns.RemoveUrl(DM.DB, creatorID, url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}
