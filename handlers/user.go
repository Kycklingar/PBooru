package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"

	"github.com/dchest/captcha"
)

type UserInfo struct {
	Gateway           string
	Limit             int
	ThumbnailSize     int
	RealThumbnailSize int
	SessionToken      string
	ThumbHover        bool
	ThumbHoverFull    bool
	CollectAlts       bool
}

func UserHandler(w http.ResponseWriter, r *http.Request) {
	u, ui := getUser(w, r)
	profile := u

	paths := splitURI(r.URL.Path)
	if len(paths) >= 2 {
		uid, err := strconv.Atoi(paths[1])
		if err != nil {
			http.Error(w, "Not a valid user id. Numerical value expected", http.StatusBadRequest)
			return
		}

		profile = DM.NewUser()
		profile.SetID(DM.DB, uid)
		profile = DM.CachedUser(profile)
	}

	type page struct {
		User        *DM.User
		UserInfo    UserInfo
		Profile     *DM.User
		RecentPosts []*DM.Post
		RecentVotes []*DM.Post
		NewMessages int
	}
	var p = page{User: u, UserInfo: ui, Profile: profile}
	u.QName(DM.DB)
	u.QFlag(DM.DB)
	profile.QName(DM.DB)
	profile.QFlag(DM.DB)

	p.RecentPosts = profile.RecentPosts(DM.DB, 5)
	for _, post := range p.RecentPosts {
		post.QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFRemoved,
		)
	}

	p.RecentVotes = profile.RecentVotes(DM.DB, 5)
	for _, post := range p.RecentVotes {
		post.QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFRemoved,
		)
	}

	p.User.QUnreadMessages(DM.DB)
	p.NewMessages = len(p.User.Messages.Unread)

	renderTemplate(w, "user", p)
}

func UserTagHistoryHandler(w http.ResponseWriter, r *http.Request) {
	paths := splitURI(r.URL.Path)
	if len(paths) < 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(paths[2])
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var page = 1
	if len(paths) >= 4 {
		page, _ = strconv.Atoi(paths[3])
		page = mm.Max(1, page)
	}

	var p TagHistoryPage
	p.User, p.UserInfo = getUser(w, r)
	p.User.QFlag(DM.DB)

	const pageLimit = 5
	var total int
	p.History, total = DM.GetUserTagHistory(pageLimit, (page-1)*pageLimit, userID)
	p.Pageinator = pageinate(total, pageLimit, page, 20)

	for _, h := range p.History {
		for _, e := range h.QETags(DM.DB) {
			e.Tag.QID(DM.DB)
			e.Tag = DM.CachedTag(e.Tag)
			e.Tag.QTag(DM.DB)
			e.Tag.QNamespace(DM.DB).QNamespace(DM.DB)
		}

		h.Post = DM.CachedPost(h.Post)

		h.Post.QMul(
			DM.DB,
			DM.PFHash,
			DM.PFThumbnails,
			DM.PFMime,
		)

		h.User.QName(DM.DB)
		h.User.QID(DM.DB)
		h.User.QFlag(DM.DB)
	}

	p.Base.Title = "Tag History"

	renderTemplate(w, "taghistory", p)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if !verifyCaptcha(w, r) {
			http.Redirect(w, r, "./#err-captcha", http.StatusSeeOther)
			return
		}

		user := DM.NewUser()

		err := user.Login(DM.DB, username, password)
		if err != nil {
			//log.Println(err)
			//http.Error(w, "Login failed", http.StatusInternalServerError)
			http.Redirect(w, r, "./#err-username", http.StatusSeeOther)
			return
		}
		//s := user.Session()
		if user.Session.Key(DM.DB) != "" {
			setCookie(w, "session", user.Session.Key(DM.DB), true)
		}

		http.Redirect(w, r, "/login/", http.StatusSeeOther)
		return

	} else {
		user, _ := getUser(w, r)
		type s struct {
			Key    string
			Expire time.Time
		}
		p := struct {
			User     *DM.User
			Key      string
			Sessions []s
		}{}
		//p.Username = user.QName(DM.DB)
		p.User = user

		user.QID(DM.DB)
		user.QName(DM.DB)
		user.QFlag(DM.DB)

		if user.QID(DM.DB) <= 0 {
			p.Key = captcha.New()
		} else {
			sessions := user.Sessions(DM.DB)
			for _, sess := range sessions {
				var ss s
				ss.Key = sess.Key(DM.DB)
				ss.Expire = sess.Expire()
				p.Sessions = append(p.Sessions, ss)
			}
		}

		renderTemplate(w, "login", p)
	}
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		sessKey := r.FormValue("session-key")
		if len(sessKey) <= 0 {
			http.Error(w, "no session key", http.StatusBadRequest)
			return
		}

		u := DM.NewUser()
		u.Session.Get(DM.DB, sessKey)

		u.Logout(DM.DB)
	} else {
		user, _ := getUser(w, r)
		if user.QID(DM.DB) > 0 {
			user.Logout(DM.DB)
		}
		removeCookie(w, "session")
	}

	http.Redirect(w, r, "/login/", http.StatusSeeOther)
}

func upgradeUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	u, _ := getUser(w, r)

	if !u.QFlag(DM.DB).Special() {
		http.Error(w, ErrPriv("Special"), http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(r.FormValue("user-id"))
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid user ID. Not an integer", http.StatusBadRequest)
		return
	}

	newFlag, err := strconv.Atoi(r.FormValue("flag"))
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid flag. Not an integer", http.StatusBadRequest)
		return
	}

	user := DM.NewUser()
	user.ID = id
	user = DM.CachedUser(user)

	if err = user.SetFlag(DM.DB, newFlag); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func getUser(w http.ResponseWriter, r *http.Request) (*DM.User, UserInfo) {
	ui := userCookies(w, r)
	user := DM.NewUser()
	user.Session.Get(DM.DB, ui.SessionToken)
	if user.QID(DM.DB) == 0 {
		removeCookie(w, "session")
	} else {
		user = DM.CachedUser(user)
	}
	return user, ui
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")
		passVerify := r.FormValue("password-verify")

		if password != passVerify {
			http.Redirect(w, r, "./#err-verify", http.StatusSeeOther)
			return
		}

		if !verifyCaptcha(w, r) {
			//http.Error(w, "Captcha was incorrect. Try again", http.StatusBadRequest)
			http.Redirect(w, r, "./#err-captcha", http.StatusSeeOther)
			return
		}

		var user = DM.NewUser()

		err := user.Register(username, password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = user.Login(DM.DB, username, password)
		if err == nil {
			if user.Session.Key(DM.DB) != "" {
				setCookie(w, "session", user.Session.Key(DM.DB), true)
			}
		}
		http.Redirect(w, r, "/login/", http.StatusSeeOther)
		return
	}

	key := captcha.New()

	renderTemplate(w, "register", key)
}

var defaultUserInfo = UserInfo{
	Limit:             defaultPostsPerPage,
	ThumbnailSize:     defaultImageSize,
	RealThumbnailSize: defaultMinThumbnailSize,
	ThumbHover:        false,
	ThumbHoverFull:    false,
}

func userCookies(w http.ResponseWriter, r *http.Request) UserInfo {
	var user = defaultUserInfo

	var ok bool
	if user.Gateway, ok = CFG.IPFSDaemonMap[r.Host]; !ok {
		user.Gateway = CFG.IPFSDaemonMap["default"]
	}

	for _, cookie := range r.Cookies() {
		var httpOnly bool
		switch cookie.Name {
		case "session":
			user.SessionToken = cookie.Value
			httpOnly = true
		case "gateway":
			user.Gateway = cookie.Value
		case "limit":
			user.Limit, _ = strconv.Atoi(cookie.Value)
			user.Limit = mm.Max(mm.Min(user.Limit, 250), 1)
		case "thumbnail_size":
			user.ThumbnailSize, _ = strconv.Atoi(cookie.Value)
			user.ThumbnailSize = mm.Max(mm.Min(user.ThumbnailSize, int(largestThumbnailSize())), 16)
		case "real_thumbnail_size":
			user.RealThumbnailSize, _ = strconv.Atoi(cookie.Value)
			user.RealThumbnailSize = mm.Max(mm.Min(user.RealThumbnailSize, int(largestThumbnailSize())), 0)
		case "thumb_hover":
			user.ThumbHover = cookie.Value == "on"
		case "thumb_hover_full":
			user.ThumbHoverFull = cookie.Value == "on"
		}

		refreshCookie(w, cookie, httpOnly)
	}

	return user
}

func refreshCookie(w http.ResponseWriter, cookie *http.Cookie, httpOnly bool) {
	setCookie(w, cookie.Name, cookie.Value, httpOnly)
}

func setCookie(w http.ResponseWriter, name, value string, httpOnly bool) {
	var expire = time.Now().Add(time.Hour * 24 * 30)
	cookie := &http.Cookie{Name: name, Value: value, Path: "/", Expires: expire, SameSite: http.SameSiteStrictMode, HttpOnly: httpOnly}
	http.SetCookie(w, cookie)
}

func removeCookie(w http.ResponseWriter, name string) {
	expire := time.Unix(0, 0)
	cookie := &http.Cookie{Name: name, Value: "", Path: "/", Expires: expire}
	http.SetCookie(w, cookie)
}
