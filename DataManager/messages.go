package DataManager

import (
	"database/sql"
	"errors"
	"log"
)

type Message struct {
	ID int

	Sender    *User
	Recipient *User

	Title string
	Text string

	Date string
}

func NewMessage()Message {
	return Message{Sender:NewUser(), Recipient:NewUser()}
}

func (m Message) Send() error {
	if m.Sender.ID <= 0 || m.Recipient.ID <= 0 {
		return errors.New("No sender/recipient specified")
	}

	_, err := DB.Exec("INSERT INTO message(sender, recipient, title, text) VALUES($1, $2, $3, $4)", m.Sender.ID, m.Recipient.ID, m.Title, m.Text)
	if err != nil {
		log.Println(err)
		return err
	}

	m.Recipient.Messages.All = nil
	m.Recipient.Messages.Unread = nil
	m.Sender.Messages.Sent = nil

	return nil
}

type messages struct {
	All    []Message
	Unread []Message
	Sent   []Message
}

func (u *User) QUnreadMessages(q querier) error {
	if u.Messages.Unread != nil {
		return nil
	}

	rows, err := q.Query("SELECT id, sender, recipient, text, date FROM message m LEFT JOIN messages_read mr ON m.id = mr.message_id WHERE m.recipient = $1 AND mr.id IS NULL", u.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	u.Messages.Unread, err = buildMessages(rows)
	if err != nil {
		log.Println(err)
	}

	return err
}

func (u *User) QAllMessages(q querier) error {
	if u.Messages.All != nil {
		return nil
	}

	rows, err := q.Query("SELECT id, sender, recipient, title, text, date FROM message m LEFT JOIN messages ms ON m.id = ms.message_id WHERE m.recipient = $1", u.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	u.Messages.All, err = buildMessages(rows)
	if err != nil {
		log.Println(err)
	}

	return err
}

func (u *User) QSentMessages(q querier) error {
	if u.Messages.Sent != nil {
		return nil
	}

	rows, err := q.Query("SELECT id, sender, recipient, title, text, date FROM message m LEFT JOIN messages_sent ms ON m.id = ms.message_id WHERE m.sender = $1", u.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	u.Messages.Sent, err = buildMessages(rows)
	if err != nil {
		log.Println(err)
	}

	return err
}

func buildMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message

	for rows.Next() {
		var m Message
		m.Sender= NewUser()
		m.Recipient = NewUser()

		if err := rows.Scan(&m.ID, &m.Sender.ID, &m.Recipient.ID, &m.Title, &m.Text, &m.Date); err != nil {
			return nil, err
		}

		m.Sender = CachedUser(m.Sender)
		m.Recipient = CachedUser(m.Recipient)

		messages = append(messages, m)
	}

	return messages, nil
}
