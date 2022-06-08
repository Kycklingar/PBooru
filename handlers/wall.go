package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/dchest/captcha"
	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/benchmark"
)

const commentEditTimeoutMinutes = 30

func CommentWallHandler(w http.ResponseWriter, r *http.Request) {
	user, uinfo := getUser(w, r)
	if r.Method == http.MethodPost {
		if CFG.EnableCommentCaptcha == captchaEveryone || (CFG.EnableCommentCaptcha == captchaAnon && user.QID(DM.DB) <= 0) {
			if !verifyCaptcha(w, r) {
				http.Error(w, "Captcha failed", http.StatusBadRequest)
				return
			}
		}

		text := r.FormValue("text")
		if len(text) < 3 || len(text) > 7500 {
			http.Error(w, "Minimum 3 characters. Maximum 7500 characters.", http.StatusBadRequest)
			return
		}

		var c DM.Comment
		err := c.Save(user.QID(DM.DB), text)
		if err != nil {
			if err.Error() == "Post does not exist" {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			log.Println(err)
		}

		http.Redirect(w, r, "/wall", http.StatusSeeOther)
	}

	bm := benchmark.Begin()
	var commMod DM.CommentCollector
	err := commMod.Get(DM.DB, 100, uinfo.Gateway)
	if internalError(w, err) {
		return
	}

	var p = struct {
		Username   string
		User       *DM.User
		Comments   []*DM.Comment
		Editable   map[int]bool
		ServerTime string
		Time       string
		Captcha    string
	}{
		Username:   user.QName(DM.DB),
		User:       user,
		Comments:   commMod.Comments,
		Editable:   make(map[int]bool),
		ServerTime: time.Now().Format(DM.Sqlite3Timestamp),
		Time:       bm.EndStr(performBenchmarks),
	}

	//p.Comments = tComments(user, commMod.Comments)

	for _, comment := range p.Comments {
		comment.User.QName(DM.DB)
		p.Editable[comment.ID] = canEditComment(commentEditTimeoutMinutes, user, comment)
	}

	if p.Username == "" {
		p.Username = "Anonymous"
	}

	if CFG.EnableCommentCaptcha == captchaEveryone || (CFG.EnableCommentCaptcha == captchaAnon && user.QID(DM.DB) <= 0) {
		p.Captcha = captcha.New()
	}

	renderTemplate(w, "comments", p)
}

func canEditComment(min time.Duration, user *DM.User, c *DM.Comment) bool {
	return user.ID > 0 && (user.QFlag(DM.DB).Special() || (c.User.ID == user.ID && time.Now().Sub(*c.Time.Time()) < time.Minute*min))
}

func editCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)

	const (
		cID = "id"
	)

	m, err := verifyInteger(r, cID)
	if badRequest(w, err) {
		return
	}

	comment, err := DM.CommentByID(m[cID])
	if internalError(w, err) {
		return
	}

	if canEditComment(commentEditTimeoutMinutes, user, comment) {
		err = comment.Edit(r.FormValue("text"))
		if internalError(w, err) {
			return
		}
	} else {
		http.Error(w, "Edit time expired", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/wall/", http.StatusSeeOther)
}

func deleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		notFoundHandler(w)
		return
	}

	user, _ := getUser(w, r)
	if !user.QFlag(DM.DB).Special() {
		permErr(w, "Special")
		return
	}

	if r.FormValue("confirm") != "on" {
		http.Error(w, "Confirmation required", http.StatusBadRequest)
		return
	}

	var commentID int
	var err error

	if commentID, err = strconv.Atoi(r.FormValue("id")); err != nil {
		http.Error(w, "id missing", http.StatusBadRequest)
		return
	}

	err = DM.DeleteComment(commentID)
	if internalError(w, err) {
		return
	}

	http.Redirect(w, r, "/wall/", http.StatusSeeOther)
}
