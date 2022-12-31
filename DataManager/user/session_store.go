package user

import (
	"database/sql"

	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/session"
	"github.com/kycklingar/PBooru/DataManager/timestamp"
)

type sessionStorage struct{ *sql.DB }

func (ss sessionStorage) Retrieve(key session.Key) (any, error) {
	var userID int
	err := ss.QueryRow(
		`SELECT user_id
		FROM sessions
		WHERE sesskey = $1`,
		key,
	).Scan(&userID)

	if err == sql.ErrNoRows {
		err = session.NotFound
	}

	return userID, err
}

func (ss sessionStorage) Put(session session.Session) error {
	_, err := ss.Exec(
		`INSERT INTO sessions(user_id, sesskey, expire)
		VALUES($1, $2, CURRENT_TIMESTAMP + INTERVAL '30 DAY')`,
		session.Value,
		session.Key,
	)
	return err
}

func (ss sessionStorage) Erase(key session.Key) error {
	_, err := ss.Exec(
		`DELETE FROM sessions
		WHERE sesskey = $1`,
		key,
	)
	return err
}

func (ss sessionStorage) UpdateExpiration(sessions ...session.Session) error {
	stmt, err := ss.Prepare(
		`UPDATE sessions
		SET expire = CURRENT_TIMESTAMP + INTERVAL '30 DAY'
		WHERE sesskey = $1`,
	)
	if err != nil {
		return err
	}

	defer stmt.Close()

	for _, session := range sessions {
		_, err = stmt.Exec(session.Key)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ss sessionStorage) CollectExpired() error {
	_, err := ss.Exec(
		`DELETE FROM sessions
		WHERE expire < CURRENT_TIMESTAMP`,
	)

	return err
}

type UserSession struct {
	Key    session.Key
	Expire timestamp.Timestamp
}

func (ss sessionStorage) userSessions(userID ID) ([]UserSession, error) {
	var sessions []UserSession
	err := db.QueryRows(
		ss,
		`SELECT sesskey, expire
		FROM sessions
		WHERE user_id = $1`,
		userID,
	)(func(scan db.Scanner) error {
		var session UserSession
		err := scan(&session.Key, &session.Expire)
		sessions = append(sessions, session)
		return err
	})

	return sessions, err
}
