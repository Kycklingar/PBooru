package DataManager

import (
	"database/sql"
	"errors"
	"log"

	"github.com/kycklingar/PBooru/DataManager/querier"
)

type Message struct {
	ID int

	Sender    *User
	Recipient *User

	Title string
	Text  string

	Date string
}

func NewMessage() Message {
	return Message{Sender: NewUser(), Recipient: NewUser()}
}

func (m Message) Send(q querier.Q) error {
	if m.Sender.ID <= 0 || m.Recipient.ID <= 0 {
		return errors.New("No sender/recipient specified")
	}

	_, err := q.Exec("INSERT INTO message(sender, recipient, title, text) VALUES($1, $2, $3, $4)", m.Sender.ID, m.Recipient.ID, m.Title, m.Text)
	if err != nil {
		log.Println(err)
		return err
	}

	m.Recipient = CachedUser(m.Recipient)

	m.Recipient.Messages.All = nil
	m.Recipient.Messages.Unread = nil
	m.Sender.Messages.Sent = nil

	return nil
}

func (m *Message) QTitle(q querier.Q) error {
	if m.ID <= 0 {
		return errors.New("No message id")
	}

	if err := q.QueryRow("SELECT title FROM message WHERE id = $1", m.ID).Scan(&m.Title); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (m *Message) QText(q querier.Q) error {
	if m.ID <= 0 {
		return errors.New("No message id")
	}

	if err := q.QueryRow("SELECT text FROM message WHERE id = $1", m.ID).Scan(&m.Text); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (m *Message) QSender(q querier.Q) error {
	if m.ID <= 0 {
		return errors.New("No message id")
	}

	return q.QueryRow("SELECT sender FROM message WHERE id = $1", m.ID).Scan(&m.Sender.ID)
}

func (m *Message) QRecipient(q querier.Q) error {
	if m.ID <= 0 {
		return errors.New("No message id")
	}

	return q.QueryRow("SELECT recipient FROM message WHERE id = $1", m.ID).Scan(&m.Recipient.ID)
}

func (m *Message) SetRead(q querier.Q) error {
	if m.ID <= 0 {
		return errors.New("No message id")
	}
	_, err := q.Exec("INSERT INTO messages_read (message_id) VALUES($1)", m.ID)
	return err
}

type messages struct {
	All    []Message
	Unread []Message
	Sent   []Message
}

func (m *messages) SetRead(q querier.Q, msg *Message) error {
	//fmt.Println(len(m.Unread), m)
	for i, _ := range m.Unread {
		//fmt.Println("Searching for:", msg.ID)
		if m.Unread[i].ID == msg.ID {
			if err := m.Unread[i].SetRead(q); err != nil {
				return err
			}
			// Remove message from unread list
			//fmt.Println("Removing message", msg.ID)
			m.Unread = append(m.Unread[:i], m.Unread[i+1:]...)
			break
		}
	}

	return nil
}

func (u *User) QUnreadMessages(q querier.Q) error {
	if u.Messages.Unread != nil {
		return nil
	}

	rows, err := q.Query("SELECT id, sender, recipient, title, text, to_char(date, 'YYYY-MM-DD HH24:MI:SS') FROM message m LEFT JOIN messages_read mr ON m.id = mr.message_id WHERE m.recipient = $1 AND mr.message_id IS NULL", u.ID)
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

func (u *User) QAllMessages(q querier.Q) error {
	if u.Messages.All != nil {
		return nil
	}

	rows, err := q.Query("SELECT id, sender, recipient, title, text, to_char(date, 'YYYY-MM-DD HH24:MI:SS') FROM message m LEFT JOIN messages ms ON m.id = ms.message_id WHERE m.recipient = $1", u.ID)
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

func (u *User) QSentMessages(q querier.Q) error {
	if u.Messages.Sent != nil {
		return nil
	}

	rows, err := q.Query("SELECT id, sender, recipient, title, text, to_char(date, 'YYYY-MM-DD HH24:MI:SS') FROM message m LEFT JOIN messages_sent ms ON m.id = ms.message_id WHERE m.sender = $1", u.ID)
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
		m.Sender = NewUser()
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
