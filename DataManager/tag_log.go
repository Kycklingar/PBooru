package DataManager

import (
	"database/sql"
)

const lMultiTags logtable = "multi_tags"

func init() {
	logTableGetFuncs[lMultiTags] = getMultiLogs
}

type logMultipleTags []logMultiTags

type logMultiTags struct {
	Tag           *Tag
	Action        lAction
	PostsAffected int

	pids []int
}

func getMultiLogs(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT m.tag_id, m.action, count(p)
		FROM log_multi_post_tags m
		JOIN log_multi_posts_affected p
		ON m.id = p.id
		WHERE log_id = $1
		GROUP BY m.tag_id, m.action
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var multilogs = make(map[lAction][]logMultiTags)

	for rows.Next() {
		var ml = logMultiTags{
			Tag: NewTag(),
		}

		err = rows.Scan(
			&ml.Tag.ID,
			&ml.Action,
			&ml.PostsAffected,
		)
		if err != nil {
			return err
		}

		mls := multilogs[ml.Action]
		mls = append(mls, ml)
		multilogs[ml.Action] = mls
	}

	log.MultiTags = multilogs

	return nil
}

func (l logMultipleTags) log(logID int, tx *sql.Tx) error {
	for _, log := range l {
		err := log.log(logID, tx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l logMultiTags) log(logID int, tx *sql.Tx) error {
	var id int

	err := tx.QueryRow(`
		INSERT INTO log_multi_post_tags(
			log_id,
			tag_id,
			action
		)
		VALUES($1,$2,$3)
		RETURNING id
		`,
		logID,
		l.Tag.ID,
		l.Action,
	).Scan(&id)
	if err != nil {
		return err
	}

	err = logAffectedPosts(logID, tx, l.pids)
	if err != nil {
		return err
	}

	stmtPosts, err := tx.Prepare(`
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
	defer stmtPosts.Close()

	for _, p := range l.pids {
		if _, err = stmtPosts.Exec(id, p); err != nil {
			return err
		}
	}

	return nil
}

func multiLogStmtFromSet(query string, tx *sql.Tx, action lAction, set tagSet) (multiLogs []logMultiTags, err error) {
	stmt, err := tx.Prepare(query)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, t := range set {
		var ml = logMultiTags{
			Tag:    t,
			Action: action,
		}

		err = func() error {
			rows, err := stmt.Query(t.ID)
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var p int
				err = rows.Scan(&p)
				if err != nil {
					return err
				}

				ml.pids = append(ml.pids, p)
			}

			return nil
		}()
		if err != nil {
			return
		}

		if len(ml.pids) > 0 {
			multiLogs = append(multiLogs, ml)
		}

	}
	return
}
