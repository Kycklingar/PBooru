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
		from, err := parseTags(fromStr, ',')
		if err != nil {
			return
		}

		to, err := parseTags(toStr, ',')
		if err != nil {
			return
		}

		if len(to) != 1 {
			err = fmt.Errorf("cannot create an alias to multiple tags: %s", toStr)
			return
		}

		if len(from) < 1 {
			err = errors.New("nothing to alias from")
			return
		}

		err = from.save(tx)
		if err != nil {
			return
		}
		err = to.save(tx)
		if err != nil {
			return
		}

		multiLogs, err := updatePtm(tx, from, to[0])
		if err != nil {
			return
		}

		updates := []func(*sql.Tx, *Tag, *Tag) error{
			updateDns,
			updateAliases,
			updateParents,
		}

		for _, f := range updates {
			for _, tf := range from {
				err = f(tx, tf, to[0])
				if err != nil {
					return
				}
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

		l.table = lAlias
		l.fn = logAlias{
			From:      from,
			To:        to[0],
			multiLogs: multiLogs,
		}.log

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
	From tagSet
	To   *Tag

	multiLogs []logMultiTags
}

func getLogAlias(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT alias_from, alias_to
		FROM log_tag_alias
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}

	log.Alias.To = NewTag()
	for rows.Next() {
		var from = NewTag()

		err = rows.Scan(&from.ID, &log.Alias.To.ID)
		if err != nil {
			return err
		}

		log.Alias.From = append(log.Alias.From, from)
	}

	return nil
}

func (l logAlias) log(logID int, tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO log_tag_alias (
			log_id,
			alias_from,
			alias_to
		)
		VALUES($1, $2, $3)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range l.From {
		_, err = stmt.Exec(logID, t.ID, l.To.ID)
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
	tos, err := tagSet{to}.upgrade(tx)
	if err != nil {
		return nil, err
	}

	var (
		multiLogs []logMultiTags

		// Queries post ids and creates and appends a multilog
		apmulf = func(query string, action lAction, set tagSet) error {
			stmt, err := tx.Prepare(query)
			if err != nil {
				return err
			}
			defer stmt.Close()

			for _, t := range set {
				var ml = logMultiTags{
					Tags: make(map[lAction]tagSet),
				}
				ml.Tags[action] = tagSet{t}

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
					return err
				}

				multiLogs = append(multiLogs, ml)

			}
			return nil
		}
	)

	err = apmulf(`
		SELECT post_id
		FROM post_tag_mappings
		WHERE tag_id = $1
		`,
		aDelete,
		from,
	)
	if err != nil {
		return nil, err
	}

	err = apmulf(
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
		aCreate,
		tos,
	)
	if err != nil {
		return nil, err
	}

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

	for _, f := range []func(querier) error{tos.recount, from.recount} {
		if err = f(tx); err != nil {
			return nil, err
		}
	}

	return multiLogs, nil
}

func updateDns(tx *sql.Tx, to, from *Tag) error {
	_, err := tx.Exec(`
		UPDATE dns_tag_mapping
		SET tag_id = $1
		WHERE tag_id = $2
		`,
		to.ID,
		from.ID,
	)

	return err
}

func updateAliases(tx *sql.Tx, from, to *Tag) error {
	// Update aliases
	_, err := tx.Exec(`
		UPDATE alias
		SET alias_to = $1
		WHERE alias_to = $2
		`,
		to.ID,
		from.ID,
	)
	if err != nil {
		return err
	}

	return err
}

func updateParents(tx *sql.Tx, to, from *Tag) error {
	_, err := tx.Exec(`
		INSERT INTO parent_tags(
			parent_id,
			child_id
		)
		SELECT parent_id, $1
		FROM parent_tags
		WHERE child_id = $2
		ON CONFLICT DO NOTHING
		`,
		to.ID,
		from.ID,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		DELETE FROM parent_tags
		WHERE child_id = $1
		`,
		from.ID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO parent_tags(
			parent_id,
			child_id
		)
		SELECT $1, child_id
		FROM parent_tags
		WHERE parent_id = $2
		ON CONFLICT DO NOTHING
		`,
		to.ID,
		from.ID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM parent_tags
		WHERE parent_id = $1
		`,
		from.ID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM parent_tags
		WHERE parent_id = child_id
		`,
	)

	return err
}
