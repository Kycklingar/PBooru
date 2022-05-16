package DataManager

import (
	"database/sql"
	"fmt"
)

const (
	lParent logtable = "tag_parent"
)

func init() {
	logTableGetFuncs[lParent] = getLogParents
}

func ParentTags(childStr, parentStr string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		parseUpgrade := func(str string) (tagSet, error) {
			set, err := parseTags(str, ',').save(tx)
			if err != nil {
				return nil, err
			}

			return set.aliases(tx)
		}

		children, err := parseUpgrade(childStr)
		if err != nil {
			return
		}

		parents, err := parseUpgrade(parentStr)
		if err != nil {
			return
		}

		if len(children) <= 0 || len(parents) <= 0 {
			err = fmt.Errorf(
				"parent tags: no children or parents present: [%s] [%s]",
				childStr,
				parentStr,
			)
			return
		}

		multiLogs, err := multiLogStmtFromSet(
			fmt.Sprintf(`
				SELECT DISTINCT hc.post_id
				FROM post_tag_mappings hc
				LEFT JOIN post_tag_mappings hp
				ON hc.post_id = hp.post_id
				AND hp.tag_id = $1
				WHERE hc.tag_id IN(%s)
				`,
				sep(",", len(children), children.strindex),
			),
			tx,
			aCreate,
			parents,
		)
		if err != nil {
			return
		}

		stmt, err := tx.Prepare(`
			INSERT INTO parent_tags (
				parent_id,
				child_id
			)
			VALUES($1, $2)
			`,
		)
		if err != nil {
			return
		}
		defer stmt.Close()

		for _, c := range children {
			for _, p := range parents {
				_, err = stmt.Exec(p.ID, c.ID)
				if err != nil {
					return
				}
			}
		}

		l.table = lParent
		l.fn = logParent{
			Children:  children,
			Parents:   parents,
			Action:    aCreate,
			multiLogs: multiLogs,
		}.log

		return
	}
}

func getLogParents(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT action, parent, child
		FROM log_tag_parent
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			parent = NewTag()
			child  = NewTag()
		)

		err := rows.Scan(&log.Parents.Action, &parent.ID, &child.ID)
		if err != nil {
			return err
		}

		log.Parents.Parents = append(log.Parents.Parents, parent)
		log.Parents.Children = append(log.Parents.Children, child)
	}

	return getMultiLogs(log, q)
}

type logParent struct {
	Children tagSet
	Parents  tagSet
	Action   lAction

	multiLogs []logMultiTags
}

func (l logParent) log(logID int, tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO log_tag_parent (
			log_id,
			action,
			parent,
			child
		)
		VALUES($1, $2, $3, $4)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range l.Children {
		for _, p := range l.Parents {
			_, err = stmt.Exec(logID, l.Action, p.ID, c.ID)
			if err != nil {
				return err
			}
		}
	}

	for _, ml := range l.multiLogs {
		err = ml.log(logID, tx)
		if err != nil {
			return err
		}
	}

	return nil
}
