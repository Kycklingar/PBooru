package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func UserPoolsHandler(w http.ResponseWriter, r *http.Request) {
	u, ui := getUser(w, r)
	profile := u

	paths := splitURI(r.URL.Path)
	if len(paths) >= 3 {
		uid, err := strconv.Atoi(paths[2])
		if err != nil {
			http.Error(w, "Not a valid user id. Numerical value expected", http.StatusBadRequest)
			return
		}

		profile = DM.NewUser()
		profile.SetID(DM.DB, uid)
	}

	type page struct {
		User     *DM.User
		UserInfo UserInfo
		Profile  *DM.User
		Pools    []*DM.Pool
	}

	var p = page{User: u, UserInfo: ui, Profile: profile, Pools: profile.QPools(DM.DB)}
	u.QName(DM.DB)
	profile.QName(DM.DB)

	for _, pool := range p.Pools {
		pool.QPosts(DM.DB)
		for _, post := range pool.Posts {
			post.Post.QThumbnails(DM.DB)
			post.Post.QHash(DM.DB)
			post.Post.QDeleted(DM.DB)
		}
	}

	renderTemplate(w, "userpools", p)
}

func UserPoolHandler(w http.ResponseWriter, r *http.Request) {
	paths := splitURI(r.URL.Path)
	if len(paths) < 3 {
		http.Error(w, "no pool id", http.StatusBadRequest)
		return
	}

	poolID, err := strconv.Atoi(paths[2])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var pool = DM.NewPool()

	pool.ID = poolID

	pool.QTitle(DM.DB)
	pool.QUser(DM.DB)
	pool.User.QName(DM.DB)
	pool.QDescription(DM.DB)
	pool.QPosts(DM.DB)
	for _, pm := range pool.Posts {
		pm.Post.QThumbnails(DM.DB)
		pm.Post.QHash(DM.DB)
		pm.Post.QDeleted(DM.DB)
	}

	type page struct {
		Pool     *DM.Pool
		User     *DM.User
		UserInfo UserInfo
		Edit     bool
		//Paginator Paginator
	}

	var p page
	p.Pool = pool
	p.User, p.UserInfo = getUser(w, r)

	// Only allow the creator of the pool to edit it
	r.ParseForm()
	p.Edit = len(r.Form["edit"]) > 0 && pool.User.QID(DM.DB) == p.User.QID(DM.DB)

	//p.Paginator = pageinate(p.Pool.TotalPosts(DM.DB), limit, page, pageCount)

	renderTemplate(w, "userpool", p)
}

func editUserPoolHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		poolID, err := strconv.Atoi(r.FormValue("pool-id"))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		u, _ := getUser(w, r)

		var pool = DM.NewPool()
		pool.ID = poolID
		if u.QID(DM.DB) != pool.QUser(DM.DB).QID(DM.DB) {
			http.Error(w, "Invalid pool, is it really yours?", http.StatusBadRequest)
			return
		}

		rem := r.Form["post-id"]

		for _, idStr := range rem {
			postID, err := strconv.Atoi(idStr)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			err = pool.RemovePost(postID)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	notFoundHandler(w, r)
}

func UserPoolAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fmt.Println(r.Method, http.MethodPost)
		http.Error(w, "Non POST methods forbidden", http.StatusBadRequest)
		return
	}

	u, _ := getUser(w, r)

	title := r.FormValue("title")
	description := r.FormValue("description")

	var pool DM.Pool
	pool.Title = title
	pool.Description = description

	pool.User = u
	err := pool.Save(DM.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func UserPoolAppendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Non POST methods forbidden", http.StatusBadRequest)
		return
	}

	postID, err := strconv.Atoi(r.FormValue("post-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	poolID, err := strconv.Atoi(r.FormValue("pool-id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var pool *DM.Pool

	u, _ := getUser(w, r)

	for _, p := range u.QPools(DM.DB) {
		if p.ID == poolID {
			pool = p
			break
		}
	}

	if pool == nil {
		http.Error(w, "You don't own a pool with that name", http.StatusBadRequest)
		return
	}

	err = pool.Add(postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}
