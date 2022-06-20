package DataManager

import (
	"database/sql"
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
	tables []logtable
	fn     logFunc
}

func (l *logger) addTable(table logtable) {
	l.tables = append(l.tables, table)
}

func (l logger) valid() bool {
	return !(len(l.tables) == 0 || l.fn == nil)
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

func (a *UserActions) Add(l ...loggingAction) {
	a.actions = append(a.actions, l...)
}

func (a *UserActions) addLogger(l logger) {
	a.Add(nullUA(l))
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
		//err = errors.New("nothing to log")
		//return err
		return nil
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

	for _, table := range l.tables(logs) {
		_, err = tx.Exec(`
			INSERT INTO logs_tables(table_name)
			VALUES($1)
			ON CONFLICT DO NOTHING
			`,
			table,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO logs_tables_altered(log_id, table_id)
			VALUES(
				$1,
				(
					SELECT id
					FROM logs_tables
					WHERE table_name = $2
				)
			)
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

func (l spine) tables(logs []logger) []logtable {
	var tablem = make(map[logtable]struct{})
	var tables []logtable

	for _, log := range logs {
		for _, table := range log.tables {
			if _, ok := tablem[table]; ok {
				continue
			}

			tablem[table] = struct{}{}
			tables = append(tables, table)
		}
	}

	return tables
}
