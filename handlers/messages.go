package handlers

import (
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
		http.Error(w, "Could not find message", http.StatusBadRequest)
		return
	}

	renderTemplate(w, "message", msg)
}

func allMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	err := u.QAllMessages(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, _ := range u.Messages.All{
		u.Messages.All[i].Recipient.QName(DM.DB)
		u.Messages.All[i].Sender.QName(DM.DB)
	}

	renderTemplate(w, "messages", u.Messages.All)
}

func sentMessagesHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := getUser(w, r)

	err := u.QSentMessages(DM.DB)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, _ := range u.Messages.Sent{
		u.Messages.Sent[i].Recipient.QName(DM.DB)
		u.Messages.Sent[i].Sender.QName(DM.DB)
	}

	renderTemplate(w, "messages", u.Messages.Sent)
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {

	var err error
	var id int

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
	}

	renderTemplate(w, "messages_write", P{recipient})
}
