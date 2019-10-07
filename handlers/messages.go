package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func messageHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	err := u.QAllMessages(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = u.QSentMessages(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uri := splitURI(r.URL.Path)
	if len(uri) < 3 {
		http.Error(w, "No message id specified", http.StatusBadRequest)
		return
	}

	msgID, err := strconv.Atoi(uri[2])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var msg *DM.Message

	for _, message := range u.Messages.All {
		if message.ID == msgID {
			msg = &message
			break
		}
	}


	if msg == nil {
		for _, message := range u.Messages.Sent{
			if message.ID == msgID {
				msg = &message
				break
			}
		}
	}
	if msg == nil {
		http.Error(w, "Could not find message", http.StatusBadRequest)
		return
	}


	u.QUnreadMessages(DM.DB)
	if err = u.Messages.SetRead(DM.DB, msg); err != nil {
		log.Println(err)
	}

	msg.Sender.QID(DM.DB)
	msg.Sender.QName(DM.DB)

	renderTemplate(w, "message", msg)
}

type messagesPage struct {
	AllMessagesCount int
	NewMessagesCount int
	SentMessagesCount int

	Messages []DM.Message
}

func allMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	err := u.QAllMessages(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, _ := range u.Messages.All {
		u.Messages.All[i].Recipient.QName(DM.DB)
		u.Messages.All[i].Sender.QName(DM.DB)
	}

	var p = messagesPage{
		AllMessagesCount: len(u.Messages.All),
		NewMessagesCount: len(u.Messages.Unread),
		SentMessagesCount: len(u.Messages.Sent),
		Messages: u.Messages.All,
	}

	renderTemplate(w, "messages", p)
}

func newMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	if err := u.QUnreadMessages(DM.DB); err != nil {
		log.Println(err)
		http.Error(w, ErrInternal, http.StatusInternalServerError)
		return
	}

	for i, _ := range u.Messages.Unread {
		u.Messages.Unread[i].Recipient.QName(DM.DB)
		u.Messages.Unread[i].Sender.QName(DM.DB)
	}

	var p = messagesPage{
		AllMessagesCount: len(u.Messages.All),
		NewMessagesCount: len(u.Messages.Unread),
		SentMessagesCount: len(u.Messages.Sent),
		Messages: u.Messages.Unread,
	}

	renderTemplate(w, "messages", p)
}

func sentMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	err := u.QSentMessages(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, _ := range u.Messages.Sent {
		u.Messages.Sent[i].Recipient.QName(DM.DB)
		u.Messages.Sent[i].Sender.QName(DM.DB)
	}

	var p = messagesPage{
		AllMessagesCount: len(u.Messages.All),
		NewMessagesCount: len(u.Messages.Unread),
		SentMessagesCount: len(u.Messages.Sent),
		Messages: u.Messages.Sent,
	}

	renderTemplate(w, "messages", p)
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		u, _ := getUser(w, r)
		var m = DM.NewMessage()
		m.Sender = u

		m.Title = r.FormValue("title")
		m.Text = r.FormValue("message")

		var err error
		m.Recipient.ID, err = strconv.Atoi(r.FormValue("recipient"))
		if err != nil {
			http.Error(w, "Recipient id invalid", http.StatusBadRequest)
			return
		}

		if err = m.Send(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		return
	}

	type P struct {
		Recipient *DM.User
		Prefill   string
	}

	var p P

	var err error
	var id int

	replyToID, _ := strconv.Atoi(r.FormValue("reply-to"))
	if replyToID > 0 {
		u, _ := getUser(w, r)
		msg, err := getReply(u, replyToID)
		if err != nil && err == msgInvalidReply {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if err != nil {
			log.Println(err)
			http.Error(w, ErrInternal, http.StatusInternalServerError)
			return
		}

		p.Prefill = fmt.Sprintf("\n\n%s Said:\n%s", msg.Sender.Name, msg.Text)
		p.Recipient = msg.Sender
	} else {
		recipient := DM.NewUser()
		if id, err = strconv.Atoi(r.FormValue("recipient")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err = recipient.SetID(DM.DB, id); err != nil {
			http.Error(w, "No recipient by that id", http.StatusBadRequest)
			return
		}

		recipient = DM.CachedUser(recipient)
		recipient.QName(DM.DB)
		p.Recipient = recipient
	}

	renderTemplate(w, "messages_write", p)
}

var msgInvalidReply = errors.New("You may only reply to messages sent to you!")

func getReply(user *DM.User, messageID int) (DM.Message, error) {
	msg := DM.NewMessage()
	msg.ID = messageID
	if err := msg.QText(DM.DB); err != nil {
		log.Println(err)
		return msg, err
	}

	msg.QRecipient(DM.DB)
	msg.Recipient = DM.CachedUser(msg.Recipient)
	if msg.Recipient.ID != user.ID {
		return msg, msgInvalidReply
	}

	msg.QSender(DM.DB)
	msg.Sender = DM.CachedUser(msg.Sender)

	msg.Sender.QName(DM.DB)

	return msg, nil
}
