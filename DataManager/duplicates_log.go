package DataManager

import (
	"database/sql"

	"github.com/kycklingar/PBooru/DataManager/query"
)

const (
	lDuplicates logtable = "duplicates"
)

func init() {
	logTableGetFuncs[lDuplicates] = getLogDuplicates
}

type logDuplicates struct {
	Action   lAction
	superior Post
	Inferior []Post
}

func (l logDuplicates) log(logid int, tx *sql.Tx) error {
	var id int
	err := tx.QueryRow(`
		INSERT INTO log_duplicates(log_id, action, post_id)
		VALUES($1, $2, $3)
		RETURNING id
		`,
		logid,
		l.Action,
		l.superior.ID,
	).Scan(&id)
	if err != nil {
		return err
	}

	err = initPostLog(tx, logid, l.superior.ID)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO log_duplicate_posts(id, dup_id)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, inf := range l.Inferior {
		err = initPostLog(tx, logid, inf.ID)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(id, inf.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func getLogDuplicates(log *Log, q querier) error {
	var id, pid int

	err := q.QueryRow(`
		SELECT id, post_id
		FROM log_duplicates
		WHERE log_id = $1
		`,
		log.ID,
	).Scan(&id, &pid)
	if err != nil {
		return err
	}

	ph := log.initPostHistory(pid)

	err = query.Rows(
		q,
		`SELECT dup_id
		FROM log_duplicate_posts
		WHERE id = $1
		`,
		id,
	)(func(scan scanner) error {
		var post = NewPost()
		err := scan(&post.ID)
		ph.Duplicates.Inferior = append(ph.Duplicates.Inferior, *post)
		return err
	})

	log.Posts[pid] = ph

	return err
}
