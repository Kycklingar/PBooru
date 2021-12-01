package DataManager

import (
	"time"
)

var logTableGetFuncs = make(map[logtable]logTableGetFunc)

type logTableGetFunc func(*Log, querier) error

type Log struct {
	ID        int
	User      *User
	Timestamp time.Time

	// Post logs
	Posts postHistoryMap

	//PostDescription   []logPostDescription
	//PostMetaData      []logPostMetaData
	//PostCreationDates []logPostCreationDates
}

func RecentLogs(q querier) ([]Log, error) {
	var logs []Log

	err := func() error {
		rows, err := q.Query(`
			SELECT log_id, user_id, timestamp
			FROM logs
			ORDER BY log_id DESC
			LIMIT 10
			`,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l = Log{
				User:  NewUser(),
				Posts: make(postHistoryMap),
			}

			if err = rows.Scan(&l.ID, &l.User.ID, &l.Timestamp); err != nil {
				return err
			}

			logs = append(logs, l)
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	for i := range logs {
		if err = logs[i].affected(q); err != nil {
			return nil, err
		}
	}

	return logs, nil
}

func (l *Log) affected(q querier) error {
	tables, err := func() ([]logtable, error) {
		rows, err := q.Query(`
			SELECT log_table
			FROM logs_affected
			WHERE log_id = $1
			`,
			l.ID,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var tables []logtable

		for rows.Next() {
			var t logtable

			if err = rows.Scan(&t); err != nil {
				return nil, err
			}

			tables = append(tables, t)
		}
		return tables, nil
	}()
	if err != nil {
		return err
	}

	for _, table := range tables {
		if err = logTableGetFuncs[table](l, q); err != nil {
			return err
		}
	}

	return nil
}
