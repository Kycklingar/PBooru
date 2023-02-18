package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	mm "github.com/kycklingar/MinMax"
	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/session"
	"github.com/kycklingar/PBooru/DataManager/user"
	"github.com/kycklingar/PBooru/DataManager/user/flag"
	"github.com/kycklingar/PBooru/DataManager/user/inbox"
	"golang.org/x/crypto/bcrypt"

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
	var (
		err     error
		profile user.Profile

		u, ui = getUser(w, r)
		paths = splitURI(r.URL.Path)
	)

	if len(paths) >= 2 {
		var uid int
		uid, err := strconv.Atoi(paths[1])
		if badRequest(w, err) {
			return
		}

		user, err := user.FromID(r.Context(), uid)
		if internalError(w, err) {
			return
		}
		profile, err = user.Profile(r.Context())
		if internalError(w, err) {
			return
		}
	} else {
		profile, err = u.Profile(r.Context())
		if internalError(w, err) {
			return
		}
	}

	var p = struct {
		User        user.User
		UserInfo    UserInfo
		Profile     user.Profile
		RecentPosts []DM.Post
		RecentVotes []DM.Post
		Inbox       inbox.Inbox
	}{
		User:     u,
		UserInfo: ui,
		Profile:  profile,
	}

	p.RecentPosts, err = DM.RecentUploads(profile.ID)
	if internalError(w, err) {
		return
	}

	p.RecentVotes, err = DM.RecentVotes(profile.ID)
	if internalError(w, err) {
		return
	}

	for i := range p.RecentPosts {
		if internalError(w, p.RecentPosts[i].QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFRemoved,
		)) {
			return
		}
	}

	for i := range p.RecentVotes {
		if internalError(w, p.RecentVotes[i].QMul(
			DM.DB,
			DM.PFCid,
			DM.PFThumbnails,
			DM.PFRemoved,
		)) {
			return
		}
	}

	if u.ID == profile.ID {
		p.Inbox, err = inbox.Of(db.Context, p.User.ID)
		if internalError(w, err) {
			return
		}
	}

	renderTemplate(w, "user", p)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if !verifyCaptcha(w, r) {
			http.Redirect(w, r, "./#err-captcha", http.StatusSeeOther)
			return
		}

		key, _, err := user.Login(r.Context(), username, password)
		if err != nil {
			if err == bcrypt.ErrMismatchedHashAndPassword || err == sql.ErrNoRows {
				http.Redirect(w, r, "./#err-username", http.StatusSeeOther)
			} else {
				internalError(w, err)
			}
			return
		}

		if key != "" {
			setCookie(w, "session", string(key), true)
		}

		http.Redirect(w, r, "/login/", http.StatusSeeOther)
		return

	} else {
		var p struct {
			User     user.User
			Key      string
			Sessions []user.UserSession
		}

		p.User, _ = getUser(w, r)

		if p.User.ID <= 0 {
			p.Key = captcha.New()
		} else {
			var err error
			p.Sessions, err = user.ActiveSessions(p.User.ID)
			if internalError(w, err) {
				return
			}
		}

		renderTemplate(w, "login", p)
	}
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		key := r.FormValue("session-key")
		if len(key) <= 0 {
			http.Error(w, "no session key", http.StatusBadRequest)
			return
		}

		user.Logout(session.Key(key))
	} else {
		ui := userCookies(w, r)
		user.Logout(session.Key(ui.SessionToken))
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

	if !u.Flag.Special() {
		http.Error(w, ErrPriv("Special"), http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(r.FormValue("user-id"))
	if badRequest(w, err) {
		return
	}

	newFlag, err := strconv.Atoi(r.FormValue("flag"))
	if badRequest(w, err) {
		return
	}

	if internalError(w, user.SetPrivileges(DM.DB, id, flag.Flag(newFlag))) {
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func getUser(w http.ResponseWriter, r *http.Request) (user.User, UserInfo) {
	ui := userCookies(w, r)

	user, err := user.FromSession(r.Context(), session.Key(ui.SessionToken))
	if err != nil {
		if err == session.NotFound {
			removeCookie(w, "session")
		} else {
			log.Println(err)
		}
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

		err := user.Register(r.Context(), username, password, DM.CFG.StdUserFlag)
		if internalError(w, err) {
			return
		}

		key, _, err := user.Login(r.Context(), username, password)
		if internalError(w, err) {
			return
		}

		setCookie(w, "session", string(key), true)

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
