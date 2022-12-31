package user

import (
	"github.com/kycklingar/PBooru/DataManager/timestamp"
)

type MessageID int

type Message struct {
	ID MessageID

	Sender    User
	Recipient User

	Title string
	Body  string

	Date timestamp.Timestamp
}
