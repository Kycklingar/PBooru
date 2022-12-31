package inbox

import (
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/user"
)

type Inbox struct {
	AllCount    int
	UnreadCount int
	SentCount   int

	userID user.ID
}

type Messages []Message

func (messages *Messages) scanner(scan db.Scanner) error {
	var m Message

	err := m.scan(scan)
	*messages = append(*messages, m)

	return err
}

func Of(q db.Q, userID user.ID) (inbox Inbox, err error) {
	inbox.userID = userID

	err = q.QueryRow(sqlAllCount, userID).Scan(&inbox.AllCount)
	if err != nil {
		return
	}
	err = q.QueryRow(sqlUnreadCount, userID).Scan(&inbox.UnreadCount)
	if err != nil {
		return
	}
	err = q.QueryRow(sqlSentCount, userID).Scan(&inbox.SentCount)

	return
}

func (inbox Inbox) All(q db.Q) (Messages, error) {
	var messages Messages
	return messages, db.QueryRows(
		q,
		sqlAllMessages,
		inbox.userID,
	)(messages.scanner)
}

func (inbox Inbox) Unread(q db.Q) (Messages, error) {
	var messages Messages
	return messages, db.QueryRows(
		q,
		sqlUnreadMessages,
		inbox.userID,
	)(messages.scanner)
}

func (inbox Inbox) Sent(q db.Q) (Messages, error) {
	var messages Messages
	return messages, db.QueryRows(
		q,
		sqlSentMessages,
		inbox.userID,
	)(messages.scanner)

}

func Send(q db.Q, sender, recipient user.ID, title, body string) error {
	_, err := q.Exec(
		`INSERT INTO message(sender, recipient, title, text)
		VALUES($1, $2, $3, $4)
		`,
		sender,
		recipient,
		title,
		body,
	)

	return err
}
