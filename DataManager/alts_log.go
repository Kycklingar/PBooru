package DataManager

import "database/sql"

const (
	lPostAlts logtable = "post_alts"
)

func init() {
	logTableGetFuncs[lPostAlts] = getLogAlts
}

type logAlts struct {
	Posts []*Post
	pids  []int
}

type logAltsSplit struct {
	a, b logAlts
}

func (l logAltsSplit) log(logID int, tx *sql.Tx) error {
	err := l.a.log(logID, tx)
	if err != nil {
		return err
	}

	return l.b.log(logID, tx)
}

func (l logAlts) log(logID int, tx *sql.Tx) error {
	var alID int

	err := tx.QueryRow(`
		INSERT INTO log_post_alts (
			log_id
		)
		VALUES($1)
		RETURNING al_id
		`,
		logID,
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
		SELECT al.al_id, post_id
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

	var lam = make(map[int]logAlts)
	for rows.Next() {
		var (
			alID int
			p    = NewPost()
		)
		err = rows.Scan(
			&alID,
			&p.ID,
		)
		if err != nil {
			return err
		}

		la := lam[alID]
		la.Posts = append(la.Posts, p)

		lam[alID] = la
	}

	for _, la := range lam {
		log.Alts = append(log.Alts, la)
	}

	return nil
}
