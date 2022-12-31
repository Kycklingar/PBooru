package inbox

import (
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/timestamp"
	"github.com/kycklingar/PBooru/DataManager/user"
)

type ID = int

type Message struct {
	ID        ID
	Sender    user.User
	Recipient user.User

	Title string
	Body  string

	Date timestamp.Timestamp
}

func (m *Message) scan(scan db.Scanner) error {
	return scan(
		&m.ID,
		&m.Sender,
		&m.Recipient,
		&m.Title,
		&m.Body,
		&m.Date,
	)
}

func MessageByID(q db.Q, id ID) (Message, error) {
	var m Message
	return m, m.scan(
		q.QueryRow(
			sqlMessageByID,
			id,
		).Scan,
	)
}

func (m Message) MarkRead(q db.Q) error {
	_, err := q.Exec(
		`INSERT INTO messages_read(message_id)
		VALUES($1)
		ON CONFLICT DO NOTHING`,
		m.ID,
	)
	return err
}
