package DataManager

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kycklingar/set"
)

const (
	lAlias logtable = "tag_alias"
)

func init() {
	logTableGetFuncs[lAlias] = getLogAlias
}

func AliasTags(fromStr, toStr string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		from := parseTags(fromStr, ',')
		to := parseTags(toStr, ',')

		if len(to.Slice) != 1 {
			err = fmt.Errorf("cannot create an alias to multiple tags: %s", toStr)
			return
		}

		if len(from.Slice) < 1 {
			err = errors.New("nothing to alias from")
			return
		}

		from, err = tagChain(from).save(tx).less(lessfnTagID).unwrap()
		if err != nil {
			return
		}
		to, err = tagChain(to).save(tx).aliases(tx).unwrap()
		if err != nil {
			return
		}

		from = set.Diff(from, to)
		if len(from.Slice) < 1 {
			err = errors.New("nothing to alias from")
			return
		}

		multiLogs, err := updatePtm(tx, from, to.Slice[0])
		if err != nil {
			return
		}

		updates := []func(*sql.Tx, set.Sorted[Tag], Tag) error{
			updateDns,
			updateAliases,
			updateParents,
		}

		for _, f := range updates {
			err = f(tx, from, to.Slice[0])
			if err != nil {
				return
			}
		}

		stmt, err := tx.Prepare(`
			INSERT INTO alias (
				alias_from,
				alias_to
			)
			VALUES($1, $2)
			`,
		)
		if err != nil {
			return
		}
		defer stmt.Close()

		for _, t := range from.Slice {
			_, err = stmt.Exec(t.ID, to.Slice[0].ID)
			if err != nil {
				return
			}
		}

		l.addTable(lAlias)
		l.fn = logAlias{
			From:      from.Slice,
			To:        to.Slice[0],
			Action:    aCreate,
			multiLogs: multiLogs,
		}.log

		return
	}
}

type logAliasMap map[int]logAlias

func (l logAliasMap) log(logID int, tx *sql.Tx) error {
	for _, log := range l {
		err := log.log(logID, tx)
		if err != nil {
			return err
		}
	}

	return nil
}

func UnaliasTags(fromStr string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		from, err := parseTagsWithID(tx, fromStr, ',')
		if err != nil {
			return
		}

		// just to make sure the aliases are valid
		stmt, err := tx.Prepare(`
			SELECT alias_to
			FROM alias
			WHERE alias_from = $1
			`,
		)
		if err != nil {
			return
		}
		defer stmt.Close()

		delStmt, err := tx.Prepare(`
			DELETE FROM alias
			WHERE alias_from = $1
			AND alias_to = $2
			`,
		)
		if err != nil {
			return
		}
		defer delStmt.Close()

		var lun = make(logAliasMap)

		for _, f := range from.Slice {
			var to Tag
			err = stmt.QueryRow(f.ID).Scan(&to.ID)
			if err != nil {
				return
			}

			_, err = delStmt.Exec(f.ID, to.ID)
			if err != nil {
				return
			}

			l := lun[to.ID]
			l.Action = aDelete
			l.To = to
			l.From = append(l.From, f)
			lun[to.ID] = l
		}

		l.addTable(lAlias)
		l.fn = lun.log

		return
	}
}

// Returns the aliased tag or input if none
func aliasedTo(q querier, tag Tag) (Tag, error) {
	var to Tag

	err := q.QueryRow(`
		SELECT id, tag, namespace
		FROM tag
		WHERE id = (
			SELECT COALESCE(alias_to, $1)
			FROM alias
			WHERE alias_from = $1
		)
		`,
		tag.ID,
	).Scan(&to.ID, &to.Tag, &to.Namespace)

	if err != nil {
		if err == sql.ErrNoRows {
			return tag, nil
		}

		return to, err
	}

	return to, nil
}

type logAlias struct {
	From   []Tag
	To     Tag
	Action lAction

	multiLogs []logMultiTags
}

func getLogAlias(log *Log, q querier) error {
	log.Aliases = make(logAliasMap)

	err := query(
		q,
		`SELECT action,
			tf.tag, tf.namespace,
			tt.tag, tt.namespace
		FROM log_tag_alias
		JOIN tag tf
		ON tf.id = alias_from
		JOIN tag tt
		ON tt.id = alias_to
		WHERE log_id = $1`,
		log.ID,
	)(func(scan scanner) error {
		var (
			action   lAction
			to, from Tag
		)

		err := scan(
			&action,
			&from.Tag, &from.Namespace,
			&to.Tag, &to.Namespace,
		)
		if err != nil {
			return err
		}

		l := log.Aliases[to.ID]
		l.To = to
		l.Action = action
		l.From = append(l.From, from)
		log.Aliases[to.ID] = l

		return nil
	})
	if err != nil {
		return err
	}

	return getMultiLogs(log, q)
}

func (l logAlias) log(logID int, tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO log_tag_alias (
			log_id,
			action,
			alias_from,
			alias_to
		)
		VALUES($1, $2, $3, $4)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range l.From {
		_, err = stmt.Exec(logID, l.Action, t.ID, l.To.ID)
		if err != nil {
			return err
		}
	}

	for _, m := range l.multiLogs {
		if err = m.log(logID, tx); err != nil {
			return err
		}
	}

	return nil
}

func updatePtm(tx *sql.Tx, from set.Sorted[Tag], to Tag) ([]logMultiTags, error) {
	tos, err := tagChain(to).upgrade(tx).unwrap()
	if err != nil {
		return nil, err
	}

	var multiLogs []logMultiTags

	ml, err := multiLogStmtFromSet(`
		SELECT post_id
		FROM post_tag_mappings
		WHERE tag_id = $1
		`,
		tx,
		aDelete,
		from,
	)
	if err != nil {
		return nil, err
	}

	multiLogs = append(multiLogs, ml...)

	ml, err = multiLogStmtFromSet(
		fmt.Sprintf(`
			SELECT DISTINCT hf.post_id
			FROM post_tag_mappings hf
			LEFT JOIN post_tag_mappings ht
			ON hf.post_id = ht.post_id
			AND ht.tag_id = $1
			WHERE hf.tag_id IN (%s)
			AND ht.post_id IS NULL
			`,
			tSetStr(from),
		),
		tx,
		aCreate,
		tos,
	)
	if err != nil {
		return nil, err
	}

	multiLogs = append(multiLogs, ml...)

	// Inserts all parents plus the alias
	_, err = tx.Exec(
		fmt.Sprintf(`
			INSERT INTO post_tag_mappings (
				post_id,
				tag_id
			)
			SELECT DISTINCT (post_id), UNNEST(ARRAY[%s])
			FROM post_tag_mappings
			WHERE tag_id IN(%s)
			ON CONFLICT DO NOTHING
			`,
			tSetStr(tos),
			tSetStr(from),
		),
	)
	if err != nil {
		return nil, err
	}

	// Delete the old tags
	_, err = tx.Exec(
		fmt.Sprintf(`
			DELETE FROM
			post_tag_mappings
			WHERE tag_id IN(%s)
			`,
			tSetStr(from),
		),
	)
	if err != nil {
		return nil, err
	}

	if err = tagChain(tos).recount(tx).purgeCountCache(tx).err; err != nil {
		return nil, err
	}
	if err = tagChain(from).recount(tx).purgeCountCache(tx).err; err != nil {
		return nil, err
	}

	return multiLogs, nil
}

func updateDns(tx *sql.Tx, from set.Sorted[Tag], to Tag) error {
	stmt, err := tx.Prepare(`
		UPDATE dns_tag_mapping
		SET tag_id = $1
		WHERE tag_id = $2
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, fromT := range from.Slice {
		_, err = stmt.Exec(to.ID, fromT.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateAliases(tx *sql.Tx, from set.Sorted[Tag], to Tag) error {
	// Update aliases
	stmt, err := tx.Prepare(`
		UPDATE alias
		SET alias_to = $1
		WHERE alias_to = $2
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, fromT := range from.Slice {
		_, err = stmt.Exec(to.ID, fromT.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateParents(tx *sql.Tx, from set.Sorted[Tag], to Tag) error {
	froms := tSetStr(from)
	_, err := tx.Exec(
		fmt.Sprintf(`
			INSERT INTO parent_tags(
				parent_id,
				child_id
			)
			(
				SELECT parent_id, $1
				FROM parent_tags
				WHERE child_id IN(%s)
				UNION
				SELECT $1, child_id
				FROM parent_tags
				WHERE parent_id IN(%s)
			)
			ON CONFLICT DO NOTHING
			`,
			froms,
			froms,
		),
		to.ID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		fmt.Sprintf(`
			DELETE FROM parent_tags
			WHERE parent_id = child_id
			OR child_id IN(%s)
			OR parent_id IN(%s)
			`,
			froms,
			froms,
		),
	)

	return err
}
