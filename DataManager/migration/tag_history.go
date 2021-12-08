package migrate

import (
	"database/sql"
	"log"
	"time"
)

type th struct {
	id        int
	userID    int
	postID    int
	timestamp time.Time
	added     []int
	removed   []int
}

func TagHistoryToUserActions(tx *sql.Tx) error {
	var (
		limit  = 100000
		offset int
	)

	for {
		batch, err := thEdits(thBatch(tx, limit, offset))
		if err != nil {
			log.Println(tx.Rollback())
			return err
		}

		offset += limit

		if len(batch) <= 0 {
			return nil
		}

		err = createUserActions(tx, batch)
		if err != nil {
			log.Println(tx.Rollback())
			return err
		}
	}

	return nil
}

func noRows(err error) error {
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

func thBatch(tx *sql.Tx, limit, offset int) (x *sql.Tx, m map[int]th, low int, high int, err error) {
	x = tx

	rows, err := tx.Query(`
		SELECT id, user_id, post_id, timestamp
		FROM tag_history
		ORDER BY id ASC
		LIMIT $1
		OFFSET $2
		`,
		limit,
		offset,
	)
	if err != nil {
		err = noRows(err)
		return
	}
	defer rows.Close()

	m = make(map[int]th)

	for rows.Next() {
		var t th

		err = rows.Scan(
			&t.id,
			&t.userID,
			&t.postID,
			&t.timestamp,
		)
		if err != nil {
			return
		}

		if t.id > high {
			high = t.id
		}
		if t.id < low {
			low = t.id
		}

		m[t.id] = t
	}

	return
}

func thEdits(tx *sql.Tx, batch map[int]th, low, high int, err error) (map[int]th, error) {
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(`
		SELECT history_id, tag_id, direction
		FROM edited_tags
		WHERE history_id >= $1
		AND history_id <= $2
		`,
		low,
		high,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			hid int
			tid int
			dir int
		)

		if err = rows.Scan(&hid, &tid, &dir); err != nil {
			return nil, err
		}

		t := batch[hid]

		// Added
		if dir == 1 {
			t.added = append(t.added, tid)
		} else if dir == -1 {
			t.removed = append(t.removed, tid)
		}

		batch[hid] = t
	}

	return batch, nil
}

const (
	actionCreate = "create"
	actionDelete = "delete"
)

func createUserActions(tx *sql.Tx, batch map[int]th) error {
	logStmt, err := tx.Prepare(`
		INSERT INTO logs (user_id, timestamp)
		VALUES($1, $2)
		RETURNING log_id
		`,
	)
	if err != nil {
		return err
	}
	defer logStmt.Close()

	editStmt, err := tx.Prepare(`
		INSERT INTO log_post_tags (
			log_id,
			action,
			post_id
		)
		VALUES($1, $2, $3)
		RETURNING id
		`,
	)
	if err != nil {
		return err
	}
	defer editStmt.Close()

	tagStmt, err := tx.Prepare(`
		INSERT INTO log_post_tags_map (
			ptid,
			tag_id
		)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer tagStmt.Close()

	postStmt, err := tx.Prepare(`
		INSERT INTO log_post(log_id, post_id)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer postStmt.Close()

	tableStmt, err := tx.Prepare(`
		INSERT INTO logs_affected(log_id, log_table)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer tableStmt.Close()

	for _, t := range batch {
		var (
			logID int
		)

		err = logStmt.QueryRow(t.userID, t.timestamp).Scan(&logID)
		if err != nil {
			return err
		}

		_, err = postStmt.Exec(logID, t.postID)
		if err != nil {
			return err
		}

		_, err = tableStmt.Exec(logID, "post_tags")
		if err != nil {
			return err
		}

		err = tagActionExec(editStmt, tagStmt, logID, t.postID, actionCreate, t.added)
		if err != nil {
			return err
		}

		err = tagActionExec(editStmt, tagStmt, logID, t.postID, actionDelete, t.removed)
		if err != nil {
			return err
		}

	}

	return nil
}

func tagActionExec(eStmt, tStmt *sql.Stmt, logID, postID int, action string, tags []int) error {
	if len(tags) <= 0 {
		return nil
	}

	var id int

	err := eStmt.QueryRow(logID, action, postID).Scan(&id)
	if err != nil {
		return err
	}

	for _, tagID := range tags {
		_, err = tStmt.Exec(id, tagID)
		if err != nil {
			return err
		}
	}

	return nil
}
