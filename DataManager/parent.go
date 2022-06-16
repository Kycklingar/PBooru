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
		children, err := parseTags(childStr, ',').
			chain().
			save(tx).
			aliases(tx).
			unwrap()

		if err != nil {
			return
		}

		parents, err := parseTags(parentStr, ',').
			chain().
			save(tx).
			aliases(tx).
			unwrap()
		if err != nil {
			return
		}

		grandParents, err := parents.chain().upgrade(tx).unwrap()
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
				AND hp.post_id IS NULL
				`,
				sep(",", len(children), children.strindex),
			),
			tx,
			aCreate,
			grandParents,
		)
		if err != nil {
			return
		}

		// Create the parent->child relationship
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

		// Insert parents and grand parents into child posts
		ins, err := tx.Prepare(
			fmt.Sprintf(`
				INSERT INTO post_tag_mappings (post_id, tag_id)
				SELECT post_id, $1
				FROM post_tag_mappings
				WHERE tag_id IN(%s)
				ON CONFLICT DO NOTHING
				`,
				sep(",", len(children), children.strindex),
			),
		)
		if err != nil {
			return
		}
		defer ins.Close()

		for _, parent := range grandParents {
			if _, err = ins.Exec(parent.ID); err != nil {
				return
			}
		}

		l.addTable(lParent)
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

	var parents, children tagSet

	for rows.Next() {
		var (
			parent = NewTag()
			child  = NewTag()
		)

		err := rows.Scan(&log.Parents.Action, &parent.ID, &child.ID)
		if err != nil {
			return err
		}

		parents = append(parents, parent)
		children = append(children, child)
	}

	log.Parents.Parents = parents.uniqueID()
	log.Parents.Children = children.uniqueID()

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
