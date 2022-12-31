package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/user"
	"github.com/kycklingar/PBooru/DataManager/user/inbox"
)

func messageHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	uri := splitURI(r.URL.Path)
	if len(uri) < 3 {
		http.Error(w, "No message id specified", http.StatusBadRequest)
		return
	}

	msgID, err := strconv.Atoi(uri[2])
	if badRequest(w, err) {
		return
	}

	msg, err := inbox.MessageByID(db.Context, msgID)
	if internalError(w, err) {
		return
	}

	if u.ID == msg.Recipient.ID {
		err = msg.MarkRead(db.Context)
		if internalError(w, err) {
			return
		}
	}

	renderTemplate(w, "message", msg)
}

type messagesPage struct {
	Inbox    inbox.Inbox
	Messages inbox.Messages
}

func allMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	var err error
	var p messagesPage

	p.Inbox, err = inbox.Of(db.Context, u.ID)
	if internalError(w, err) {
		return
	}

	p.Messages, err = p.Inbox.All(db.Context)
	if internalError(w, err) {
		return
	}

	renderTemplate(w, "messages", p)
}

func newMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	var (
		p   messagesPage
		err error
	)
	p.Inbox, err = inbox.Of(db.Context, u.ID)
	if internalError(w, err) {
		return
	}
	p.Messages, err = p.Inbox.Unread(db.Context)
	if internalError(w, err) {
		return
	}

	renderTemplate(w, "messages", p)
}

func sentMessagesHandler(w http.ResponseWriter, r *http.Request) {
	var (
		u, _ = getUser(w, r)
		p    messagesPage
		err  error
	)

	p.Inbox, err = inbox.Of(db.Context, u.ID)
	if internalError(w, err) {
		return
	}
	p.Messages, err = p.Inbox.Sent(db.Context)
	if internalError(w, err) {
		return
	}

	renderTemplate(w, "messages", p)
}

func postMessageHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	title := r.FormValue("title")
	body := r.FormValue("message")

	recipient, err := strconv.Atoi(r.FormValue("recipient"))
	if badRequest(w, err) {
		return
	}

	inbox.Send(db.Context, u.ID, recipient, title, body)

	sentMessagesHandler(w, r)
	return
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		postMessageHandler(w, r)
		return
	}

	var err error
	var id int
	var p struct {
		Recipient    user.User
		PrefillTitle string
		Prefill      string
	}

	replyToID, _ := strconv.Atoi(r.FormValue("reply-to"))
	if replyToID > 0 {
		u, _ := getUser(w, r)
		msg, err := getReply(u, replyToID)
		if err != nil && err == msgInvalidReply {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if internalError(w, err) {
			return

		}

		p.Prefill = fmt.Sprintf("\n\n%s Said:\n%s", msg.Sender.Name, msg.Body)

		p.PrefillTitle = fmt.Sprint("RE: ", func() string {
			if len(msg.Title) <= 0 {
				return "No Subject"
			}
			return msg.Title
		}())

		p.Recipient = msg.Sender
	} else {
		id, err = strconv.Atoi(r.FormValue("recipient"))
		if badRequest(w, err) {
			return
		}
		p.Recipient, err = user.FromID(r.Context(), id)
		if internalError(w, err) {
			return
		}
	}

	renderTemplate(w, "messages_write", p)
}

var msgInvalidReply = errors.New("You may only reply to messages sent to you!")

func getReply(user user.User, messageID inbox.ID) (inbox.Message, error) {
	msg, err := inbox.MessageByID(db.Context, messageID)
	if err != nil {
		return msg, err
	}

	if user.ID != msg.Recipient.ID {
		err = msgInvalidReply
	}

	return msg, err
}
