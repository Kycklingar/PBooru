package DataManager

import "database/sql"

const (
	lPostAlts logtable = "post_alts"
)

func init() {
	logTableGetFuncs[lPostAlts] = getLogAlts
}

type logAlts struct {
	AltGroup int
	Posts    []*Post
	pids     []int
}

func (l logAlts) log(logID int, tx *sql.Tx) error {
	var alID int

	err := tx.QueryRow(`
		INSERT INTO log_post_alts (
			log_id,
			new_alt
		)
		VALUES($1, $2)
		RETURNING al_id
		`,
		logID,
		l.AltGroup,
	).Scan(&alID)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO log_post_alt_posts(
			al_id,
			post_id
		)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pid := range l.pids {
		if _, err = stmt.Exec(alID, pid); err != nil {
			return err
		}

		if err = initPostLog(tx, logID, pid); err != nil {
			return err
		}
	}

	return nil
}

func getLogAlts(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT new_alt, post_id
		FROM log_post_alts al
		LEFT JOIN log_post_alt_posts p
		ON al.al_id = p.al_id
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p = NewPost()
		err = rows.Scan(&log.Alts.AltGroup, &p.ID)
		if err != nil {
			return err
		}

		log.Alts.Posts = append(log.Alts.Posts, p)
	}

	return nil
}
