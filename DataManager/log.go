package DataManager

import (
	"database/sql"
	"errors"
	"time"
)

const (
	aCreate lAction = "create"
	aModify lAction = "modify"
	aDelete lAction = "delete"
)

type loggingAction func(tx *sql.Tx) (logger, error)
type logFunc func(logid int, tx *sql.Tx) error
type logtable string
type lAction string

type logger struct {
	table logtable
	fn    logFunc
}

func (l logger) valid() bool {
	return !(l.table == "" || l.fn == nil)
}

type spine struct {
	Id        int
	User      *User
	Timestamp time.Time
}

type UserActions struct {
	log spine

	actions []loggingAction
}

func newLog(user *User) spine {
	return spine{User: user}
}

func UserAction(user *User) *UserActions {
	return &UserActions{
		log: newLog(user),
	}
}

func nullUA(l logger) loggingAction {
	return func(*sql.Tx) (logger, error) {
		return l, nil
	}
}

func (a *UserActions) Add(l loggingAction) {
	a.actions = append(a.actions, l)
}

func (a UserActions) Exec() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer commitOrDie(tx, &err)

	err = a.exec(tx)
	return err
}

func (a UserActions) exec(tx *sql.Tx) error {
	var (
		err  error
		logs []logger
	)

	for _, act := range a.actions {
		var l logger
		l, err = act(tx)
		if err != nil {
			return err
		}

		if l.valid() {
			logs = append(logs, l)
		}
	}

	if len(logs) <= 0 {
		err = errors.New("nothing to log")
		return err
	}

	err = a.log.insert(tx, logs)
	if err != nil {
		return err
	}

	return nil
}

func (l spine) insert(tx *sql.Tx, logs []logger) error {
	var logID int

	err := tx.QueryRow(`
		INSERT INTO logs (user_id)
		VALUES($1)
		RETURNING log_id
		`,
		l.User.ID,
	).Scan(&logID)
	if err != nil {
		return err
	}

	for _, table := range l.affected(logs) {
		_, err = tx.Exec(`
			INSERT INTO logs_affected(log_id, log_table)
			VALUES($1, $2)
			`,
			logID,
			table,
		)
		if err != nil {
			return err
		}
	}

	for _, log := range logs {
		if err = log.fn(logID, tx); err != nil {
			return err
		}
	}

	return err
}

func (l spine) affected(logs []logger) []logtable {
	var tablem = make(map[logtable]struct{})
	var tables []logtable

	for _, log := range logs {
		if _, ok := tablem[log.table]; ok {
			continue
		}

		tablem[log.table] = struct{}{}
		tables = append(tables, log.table)
	}

	return tables
}
