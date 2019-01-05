package handlers

import (
	"fmt"
	DM "github.com/kycklingar/PBooru/DataManager"
	"net/http"
	"strconv"
	"time"

	"github.com/dchest/captcha"
)

type User struct {
	ID   int
	Name string
	Flag int
}

type UserInfo struct {
	IpfsDaemon       string
	Limit            int
	ImageSize        int
	MinThumbnailSize int
	SessionToken     string
	ThumbHover       bool
	ThumbHoverFull   bool
}

func tUser(u *DM.User) User {
	return User{u.QID(DM.DB), u.QName(DM.DB), u.QFlag(DM.DB)}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if !verifyCaptcha(w, r) {
			http.Error(w, "Captcha was incorrect. Try again!", http.StatusBadRequest)
			return
		}

		user := DM.NewUser()

		err := user.Login(DM.DB, username, password)
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, "Login failed", http.StatusInternalServerError)
			return
		}
		//s := user.Session()
		if user.Session.Key(DM.DB) != "" {
			setC(w, "session", user.Session.Key(DM.DB))
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
			Username string
			Key      string
			Sessions []s
		}{}
		p.Username = user.QName(DM.DB)

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
	user, _ := getUser(w, r)

	setC(w, "session", "")

	if user.QID(DM.DB) > 0 {
		user.Logout(DM.DB)
	}

	http.Redirect(w, r, "/login/", http.StatusSeeOther)
}

func getUser(w http.ResponseWriter, r *http.Request) (*DM.User, UserInfo) {
	ui := userCookies(w, r)
	user := DM.NewUser()
	user.Session.Get(DM.DB, ui.SessionToken)
	if user.QID(DM.DB) == 0 {
		remC(w, "session")
	}
	return user, ui
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if !verifyCaptcha(w, r) {
			http.Error(w, "Captcha was incorrect. Try again", http.StatusBadRequest)
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
				setC(w, "session", user.Session.Key(DM.DB))
			}
		}
		http.Redirect(w, r, "/login/", http.StatusSeeOther)
		return
	}

	key := captcha.New()

	renderTemplate(w, "register", key)
}

func userCookies(w http.ResponseWriter, r *http.Request) UserInfo {
	var user UserInfo

	cookie, err := r.Cookie("daemon")
	if err != nil {
		daemon := CFG.IPFSDaemonMap[r.Host]
		if daemon == "" {
			daemon = CFG.IPFSDaemonMap["default"]
		}
		setC(w, "daemon", daemon)
		user.IpfsDaemon = daemon
	} else {
		user.IpfsDaemon = cookie.Value
		updateCookie(w, cookie)
	}

	cookie, err = r.Cookie("limit")
	//fmt.Println(cookie)

	if err != nil {
		setC(w, "limit", strconv.Itoa(defaultPostsPerPage))
		user.Limit = defaultPostsPerPage
	} else {
		val, err := strconv.Atoi(cookie.Value)
		if err != nil {
			val = defaultPostsPerPage
		}
		user.Limit = min(max(val, 250), 1)
		//user.Limit = val
		updateCookie(w, cookie)
	}

	cookie, err = r.Cookie("ImageSize")
	if err != nil {
		setC(w, "ImageSize", strconv.Itoa(defaultImageSize))
		user.ImageSize = defaultImageSize
	} else {
		val, err := strconv.Atoi(cookie.Value)
		if err != nil {
			val = defaultImageSize
		}
		if val < 1 {
			val = 1
		} else if val > 1024 {
			val = 1024
		}
		user.ImageSize = val
		updateCookie(w, cookie)
	}

	cookie, err = r.Cookie("MinThumbnailSize")
	if err != nil {
		setC(w, "MinThumbnailSize", strconv.Itoa(defaultMinThumbnailSize))
		user.MinThumbnailSize = defaultMinThumbnailSize
	} else {
		val, err := strconv.Atoi(cookie.Value)
		if err != nil {
			val = defaultMinThumbnailSize
		}
		user.MinThumbnailSize = val
		updateCookie(w, cookie)
	}

	cookie, err = r.Cookie("session")
	if err != nil {
		user.SessionToken = ""
	} else {
		user.SessionToken = cookie.Value
		updateCookie(w, cookie)
	}

	cookie, err = r.Cookie("thumbhover")
	if err != nil {
		setC(w, "thumbhover", "false")
		user.ThumbHover = false
	} else {
		if cookie.Value == "true" {
			user.ThumbHover = true
		} else {
			user.ThumbHover = false
		}
		updateCookie(w, cookie)
	}

	cookie, err = r.Cookie("thumbhoverfull")
	if err != nil {
		setC(w, "thumbhoverfull", "false")
		user.ThumbHoverFull = false
	} else {
		if cookie.Value == "true" {
			user.ThumbHoverFull = true
		} else {
			user.ThumbHoverFull = false
		}
		updateCookie(w, cookie)
	}
	return user
}

func updateCookie(w http.ResponseWriter, cookie *http.Cookie) {
	// var expire = time.Now().Add(time.Hour * 24 * 30)
	// cookie.Expires = expire
	// http.SetCookie(w, cookie)
	setC(w, cookie.Name, cookie.Value)
	return
}

func setC(w http.ResponseWriter, name, value string) {
	var expire = time.Now().Add(time.Hour * 24 * 30)
	cookie := &http.Cookie{Name: name, Value: value, Path: "/", Expires: expire}
	http.SetCookie(w, cookie)
}

func remC(w http.ResponseWriter, name string) {
	expire := time.Unix(0, 0)
	cookie := &http.Cookie{Name: name, Value: "", Path: "/", Expires: expire}
	http.SetCookie(w, cookie)
}
