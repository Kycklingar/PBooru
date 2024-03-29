package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	dns "github.com/kycklingar/PBooru/DataManager/dns"
	"github.com/kycklingar/PBooru/DataManager/user"
)

func specialMM(handle func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := getUser(w, r)
		if !user.Flag.Special() {
			permErr(w, "Special")
			return
		}
		handle(w, r)
	}
}

func dnsHandler(w http.ResponseWriter, r *http.Request) {
	_, ui := getUser(w, r)

	const limit = 10
	offset, _ := strconv.Atoi(r.FormValue("offset"))

	dc, err := DM.ListDnsCreators(limit, offset)
	if internalError(w, err) {
		return
	}

	var p = struct {
		UserInfo   UserInfo
		Creators   []DM.DnsCreator
		NextOffset int
		PrevOffset int
	}{
		UserInfo:   ui,
		Creators:   dc,
		NextOffset: offset + limit,
		PrevOffset: offset - limit,
	}

	renderTemplate(w, "dns", p)
}

func dnsCreatorHandler(w http.ResponseWriter, r *http.Request) {
	type tag struct {
		Enabled bool
		Tag     dns.Tag
	}
	var p struct {
		UserInfo UserInfo
		Creator  DM.DnsCreator
		Domains  []dns.Domain
		Tags     []tag
		CanEdit  bool
	}

	var user user.User

	user, p.UserInfo = getUser(w, r)

	p.CanEdit = user.Flag.Special()

	uri := uriSplitter(r)
	id, err := uri.getIntAtIndex(1)
	if badRequest(w, err) {
		return
	}

	p.Creator, err = DM.GetDnsCreator(id)
	if internalError(w, err) {
		return
	}

	p.Domains, err = dns.Domains(DM.DB)
	if internalError(w, err) {
		return
	}

	tags, err := dns.AllTags(DM.DB)
	if internalError(w, err) {
		return
	}

	for _, atag := range tags {
		var t = tag{Tag: atag}

		for _, ctag := range p.Creator.Tags {
			if atag.Id == ctag.Id {
				t.Enabled = true
				break
			}
		}

		p.Tags = append(p.Tags, t)
	}

	renderTemplate(w, "dns_creator", p)
}

func dnsNewCreator(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	cid, err := dns.NewCreator(DM.DB, name)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", cid), http.StatusSeeOther)
}

func dnsEditCreatorName(w http.ResponseWriter, r *http.Request) {
	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if badRequest(w, err) {
		return
	}

	name := r.FormValue("name")

	err = dns.CreatorEditName(DM.DB, creatorID, name)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsNewBanner(w http.ResponseWriter, r *http.Request) {
	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if badRequest(w, err) {
		return
	}

	bannerType := r.FormValue("banner-type")
	file, _, err := r.FormFile("file")
	if internalError(w, err) {
		return
	}

	if internalError(w, DM.DnsNewBanner(file, creatorID, bannerType)) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsEditHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err error
		p   struct {
			Tags    []dns.Tag
			Domains []dns.Domain
		}
	)

	user, _ := getUser(w, r)

	if !user.Flag.Special() {
		permErr(w, "Special")
		return
	}

	p.Tags, err = dns.AllTags(DM.DB)
	if internalError(w, err) {
		return
	}

	p.Domains, err = dns.Domains(DM.DB)
	if internalError(w, err) {
		return
	}

	renderTemplate(w, "dns_edit", p)
}

func dnsAddUrl(w http.ResponseWriter, r *http.Request) {
	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if badRequest(w, err) {
		return
	}

	url := r.FormValue("url")
	domain := r.FormValue("domain")

	if internalError(w, dns.AddUrl(DM.DB, creatorID, url, domain)) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsRemoveUrl(w http.ResponseWriter, r *http.Request) {
	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if badRequest(w, err) {
		return
	}

	url := r.FormValue("url")

	err = dns.RemoveUrl(DM.DB, creatorID, url)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsEditCreatorTags(w http.ResponseWriter, r *http.Request) {
	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if badRequest(w, err) {
		return
	}

	enabledTags := r.Form["tags"]

	err = dns.EditTags(DM.DB, creatorID, enabledTags)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsMapTag(w http.ResponseWriter, r *http.Request) {
	tagstr := r.FormValue("tag")
	creatorID, err := strconv.Atoi(r.FormValue("creator-id"))
	if badRequest(w, err) {
		return
	}

	err = DM.DnsMapTag(creatorID, tagstr)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dns/%d", creatorID), http.StatusSeeOther)
}

func dnsTagCreate(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	name := r.FormValue("name")
	descr := r.FormValue("description")
	score, err := strconv.Atoi(r.FormValue("score"))
	if badRequest(w, err) {
		return
	}

	err = dns.CreateTag(DM.DB, id, name, descr, score)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func dnsTagEdit(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	name := r.FormValue("name")
	descr := r.FormValue("description")
	score, err := strconv.Atoi(r.FormValue("score"))
	if badRequest(w, err) {
		return
	}

	err = dns.UpdateTag(DM.DB, id, name, descr, score)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func dnsNewDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.FormValue("domain")
	icon := r.FormValue("icon")

	err := dns.DomainNew(DM.DB, domain, icon)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, "/dns/edit", http.StatusSeeOther)
}

func dnsEditDomain(w http.ResponseWriter, r *http.Request) {
	domID, err := strconv.Atoi(r.FormValue("id"))
	if badRequest(w, err) {
		return
	}

	domain := r.FormValue("domain")
	icon := r.FormValue("icon")

	err = dns.DomainEdit(DM.DB, domID, domain, icon)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, "/dns/edit", http.StatusSeeOther)

}
