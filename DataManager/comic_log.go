package DataManager

import "database/sql"

const lComic logtable = "comic"

func init() {
	logTableGetFuncs[lComic] = getLogComic
}

type logComic struct {
	Action lAction
	ID     int
	Title  string
}

func getLogComic(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT id, action, title
		FROM log_comics
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var l logComic
		err = rows.Scan(&l.ID, &l.Action, &l.Title)
		if err != nil {
			return err
		}
		log.Comic = l
	}

	return nil
}

func (l logComic) log(logID int, tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO log_comics (
			log_id,
			id,
			title,
			action
		)
		VALUES($1, $2, $3, $4)
		`,
		logID,
		l.ID,
		l.Title,
		l.Action,
	)

	return err
}
