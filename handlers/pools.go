package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/user"
	"github.com/kycklingar/PBooru/DataManager/user/pool"
)

func UserPoolsHandler(w http.ResponseWriter, r *http.Request) {
	u, ui := getUser(w, r)
	profile := u

	paths := splitURI(r.URL.Path)
	if len(paths) >= 3 {
		uid, err := strconv.Atoi(paths[2])
		if badRequest(w, err) {
			return
		}

		profile, err = user.FromID(r.Context(), uid)
		if internalError(w, err) {
			return
		}
	}
	var (
		err error
		p   = struct {
			User     user.User
			UserInfo UserInfo
			Profile  user.User
			Pools    []DM.Pool
		}{
			User:     u,
			UserInfo: ui,
			Profile:  profile,
		}
	)

	p.Pools, err = DM.PoolsOfUser(r.Context(), profile.ID)
	if internalError(w, err) {
		return
	}

	for i := range p.Pools {
		p.Pools[i].QPosts(DM.DB)
		for _, mapping := range p.Pools[i].Posts {
			mapping.Post.QMul(
				DM.DB,
				DM.PFThumbnails,
				DM.PFCid,
				DM.PFRemoved,
			)
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
	if badRequest(w, err) {
		return
	}

	var p struct {
		Pool     DM.Pool
		User     user.User
		UserInfo UserInfo
		Edit     bool
	}

	p.Pool, err = DM.PoolFromID(r.Context(), pool.ID(poolID))
	if internalError(w, err) {
		return
	}

	p.Pool.QPosts(DM.DB)
	for _, pm := range p.Pool.Posts {
		pm.Post.QMul(
			DM.DB,
			DM.PFThumbnails,
			DM.PFCid,
			DM.PFRemoved,
		)
	}

	//p.Pool = pool
	p.User, p.UserInfo = getUser(w, r)

	// Only allow the creator of the pool to edit it
	r.ParseForm()
	p.Edit = len(r.Form["edit"]) > 0 && p.Pool.User.ID == p.User.ID

	//p.Paginator = pageinate(p.Pool.TotalPosts(DM.DB), limit, page, pageCount)

	renderTemplate(w, "userpool", p)
}

func editUserPoolHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		poolID, err := strconv.Atoi(r.FormValue("pool-id"))
		if badRequest(w, err) {
			return
		}

		u, _ := getUser(w, r)

		Pool, err := pool.FromID(r.Context(), pool.ID(poolID))
		if internalError(w, err) {
			return
		}

		if u.ID != Pool.User.ID {
			http.Error(w, "Invalid pool, is it really yours?", http.StatusBadRequest)
			return
		}

		rem := r.Form["post-id"]

		for _, idStr := range rem {
			postID, err := strconv.Atoi(idStr)
			if badRequest(w, err) {
				return
			}
			err = Pool.RemovePost(r.Context(), postID)
			if internalError(w, err) {
				return
			}
		}

		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	notFoundHandler(w)
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

	err := pool.Create(r.Context(), u.ID, title, description)
	if internalError(w, err) {
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
	if badRequest(w, err) {
		return
	}
	if badRequest(w, err) {
		return
	}

	poolID, err := strconv.Atoi(r.FormValue("pool-id"))
	if badRequest(w, err) {
		return
	}

	u, _ := getUser(w, r)

	pool, err := pool.FromID(r.Context(), pool.ID(poolID))
	if internalError(w, err) {
		return
	}

	if pool.User.ID != u.ID {
		http.Error(w, "Not your pool", http.StatusBadRequest)
		return
	}

	err = pool.AddPost(r.Context(), postID)
	if badRequest(w, err) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}
