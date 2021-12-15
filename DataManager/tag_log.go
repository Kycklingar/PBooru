package DataManager

import "database/sql"

type logMultiTags struct {
	Tags          map[lAction]tagSet
	PostsAffected int

	pids []int
}

func (l logMultiTags) log(logID int, tx *sql.Tx) error {
	var id int

	err := tx.QueryRow(`
		INSERT INTO log_multi_post_tags(
			log_id
		)
		VALUES($1)
		RETURNING id
		`,
		logID,
	).Scan(&id)
	if err != nil {
		return err
	}

	err = initPostsLog(logID, tx, l.pids)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO log_multi_posts_affected (
			id,
			post_id
		)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range l.pids {
		if _, err = stmt.Exec(id, p); err != nil {
			return err
		}
	}

	return nil
}
