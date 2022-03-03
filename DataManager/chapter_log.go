package DataManager

import "database/sql"

const lChapter logtable = "comic_chapter"

func init() {
	logTableGetFuncs[lChapter] = getLogChapter
}

type logChapter struct {
	Action lAction
	ID     int
	Order  int
	Title  string
}

func getLogChapter(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT action, chapter_id, c_order, title
		FROM log_chapters
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var lc logChapter
		err = rows.Scan(
			&lc.Action,
			&lc.ID,
			&lc.Order,
			&lc.Title,
		)
		if err != nil {
			return err
		}

		log.Chapters = append(log.Chapters, lc)
	}

	return nil
}

func (l logChapter) log(logID int, tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO log_chapters (
			log_id,
			chapter_id,
			action,
			c_order,
			title
		)
		VALUES($1, $2, $3, $4, $5)
		`,
		logID,
		l.ID,
		l.Action,
		l.Order,
		l.Title,
	)
	if err != nil {
		return err
	}

	return nil
}
