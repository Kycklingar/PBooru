package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
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

		if len(to) != 1 {
			err = fmt.Errorf("cannot create an alias to multiple tags: %s", toStr)
			return
		}

		if len(from) < 1 {
			err = errors.New("nothing to alias from")
			return
		}

		err = from.chain().save(tx).err
		if err != nil {
			return
		}
		err = to.chain().save(tx).aliases(tx).err
		if err != nil {
			return
		}

		from = from.diffID(to)
		if len(from) < 1 {
			err = errors.New("nothing to alias from")
			return
		}

		multiLogs, err := updatePtm(tx, from, to[0])
		if err != nil {
			return
		}

		updates := []func(*sql.Tx, tagSet, *Tag) error{
			updateDns,
			updateAliases,
			updateParents,
		}

		for _, f := range updates {
			err = f(tx, from, to[0])
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

		for _, t := range from {
			_, err = stmt.Exec(t.ID, to[0].ID)
			if err != nil {
				return
			}
		}

		l.addTable(lAlias)
		l.fn = logAlias{
			From:      from,
			To:        to[0],
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

		for _, f := range from.unique() {
			var to = NewTag()
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
func aliasedTo(q querier, tag *Tag) (*Tag, error) {
	var to = NewTag()

	err := q.QueryRow(`
		SELECT t.id, t.tag, n.nspace
		FROM tags t
		JOIN namespaces n
		ON t.namespace_id = n.id
		WHERE t.id = (
			SELECT COALESCE(alias_to, $1)
			FROM alias
			WHERE alias_from = $1
		)
		`,
		tag.ID,
	).Scan(&to.ID, &to.Tag, &to.Namespace.Namespace)

	if err != nil {
		if err == sql.ErrNoRows {
			return tag, nil
		}

		return nil, err
	}

	return to, nil
}

type logAlias struct {
	From   tagSet
	To     *Tag
	Action lAction

	multiLogs []logMultiTags
}

func getLogAlias(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT action, alias_from, alias_to
		FROM log_tag_alias
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}

	var la = make(logAliasMap)

	for rows.Next() {
		var (
			action lAction
			to     = NewTag()
			from   = NewTag()
		)

		err = rows.Scan(&action, &from.ID, &to.ID)
		if err != nil {
			return err
		}

		l := la[to.ID]
		l.To = to
		l.Action = action
		l.From = append(l.From, from)
		la[to.ID] = l
	}

	log.Aliases = la

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

func NewAlias() *Alias {
	var a Alias
	a.Tag = NewTag()
	a.To = NewTag()
	return &a
}

type Alias struct {
	Tag  *Tag
	From []*Tag
	To   *Tag
}

func (a *Alias) QFrom(q querier) []*Tag {
	if a.From != nil {
		return a.From
	}
	if a.Tag.QID(q) == 0 {
		return nil
	}
	rows, err := q.Query("SELECT alias_from FROM alias WHERE alias_to=$1", a.Tag.QID(q))
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		t := NewTag()
		//t.SetQ(q)
		err = rows.Scan(&t.ID)
		if err != nil {
			log.Print(err)
			return nil
		}
		a.From = append(a.From, t)
	}
	return a.From
}

func (a *Alias) QTo(q querier) (*Tag, error) {
	if a.Tag.QID(q) == 0 {
		return a.To, nil
	}
	if a.To.QID(q) != 0 {
		return a.To, nil
	}

	err := q.QueryRow("SELECT alias_to FROM alias WHERE alias_from=$1", a.Tag.QID(q)).Scan(&a.To.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return nil, err
	}

	return a.To, nil
}

func updatePtm(tx *sql.Tx, from tagSet, to *Tag) ([]logMultiTags, error) {
	tos, err := tagSet{to}.chain().upgrade(tx).unwrap()
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
			sep(",", len(from), from.strindex),
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
			sep(",", len(tos), tos.strindex),
			sep(",", len(from), from.strindex),
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
			sep(",", len(from), from.strindex),
		),
	)
	if err != nil {
		return nil, err
	}

	for _, f := range []func(querier) tagSetChain{tos.chain().recount, from.chain().recount} {
		if err = f(tx).err; err != nil {
			return nil, err
		}
	}

	return multiLogs, nil
}

func updateDns(tx *sql.Tx, from tagSet, to *Tag) error {
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

	for _, fromT := range from {
		_, err = stmt.Exec(to.ID, fromT.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateAliases(tx *sql.Tx, from tagSet, to *Tag) error {
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

	for _, fromT := range from {
		_, err = stmt.Exec(to.ID, fromT.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateParents(tx *sql.Tx, from tagSet, to *Tag) error {
	froms := sep(",", len(from), from.strindex)
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
