package DataManager

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kycklingar/PBooru/DataManager/query"
	"github.com/kycklingar/set"
)

const (
	lParent logtable = "tag_parent"
)

func init() {
	logTableGetFuncs[lParent] = getLogParents
}

func ParentTags(childStr, parentStr string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		children, err := tsChain(parseTags(childStr)).
			save(tx).
			aliases(tx).
			unwrap()

		if err != nil {
			return
		}

		parents, err := tsChain(parseTags(parentStr)).
			save(tx).
			aliases(tx).
			unwrap()
		if err != nil {
			return
		}

		grandParents, err := tagChain(parents).copy().upgrade(tx).unwrap()
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
				tSetStr(children),
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
				tSetStr(children),
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

		err = tagChain(grandParents).recount(tx).purgeCountCache(tx).err

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

func UnparentTags(childStr, parentStr string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		parents, err := tsChain(parseTags(parentStr)).qids(tx).unwrap()
		if err != nil {
			return
		}

		children, err := tsChain(parseTags(childStr)).qids(tx).unwrap()
		if err != nil {
			return
		}

		childParents, err := tagChain(children).copy().parents(tx).unwrap()
		if err != nil {
			return
		}

		childParents = set.Diff(childParents, children)
		parents = set.Intersection(parents, childParents)

		if len(parents) <= 0 || len(children) <= 0 {
			err = errors.New("not childs parents")
			return
		}

		_, err = tx.Exec(
			fmt.Sprintf(`
				DELETE FROM
				parent_tags
				WHERE parent_id IN(%s)
				AND child_id IN(%s)
				`,
				tSetStr(parents),
				tSetStr(children),
			),
		)
		if err != nil {
			return
		}

		l.addTable(lParent)
		l.fn = logParent{
			Children: children,
			Parents:  parents,
			Action:   aDelete,
		}.log

		return
	}
}

func getLogParents(log *Log, q querier) error {
	parents := set.New[Tag]()
	children := set.New[Tag]()

	err := query.Rows(
		q,
		`SELECT action,
			tp.id, tp.tag, tp.namespace,
			tc.id, tc.tag, tc.namespace
		FROM log_tag_parent
		JOIN tag tp
		ON parent = tp.id
		JOIN tag tc
		ON child = tc.id
		WHERE log_id = $1
		`,
		log.ID,
	)(func(scan scanner) error {
		var parent, child Tag
		err := scan(
			&log.Parents.Action,
			&parent.ID, &parent.Tag, &parent.Namespace,
			&child.ID, &child.Tag, &child.Namespace,
		)
		parents.Set(parent)
		children.Set(child)
		return err
	})
	if err != nil {
		return err
	}

	log.Parents.Parents = parents
	log.Parents.Children = children

	return getMultiLogs(log, q)
}

type logParent struct {
	Children []Tag
	Parents  []Tag
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
