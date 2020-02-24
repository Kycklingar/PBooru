package DataManager

import (
	"database/sql"
	"errors"
	"log"
)

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

func (a *Alias) Save() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	if a.Tag.QID(tx) == 0 {
		err = a.Tag.Save(tx)
		if err != nil {
			return err
		}
	}
	if a.To.QID(tx) == 0 {
		err = a.To.Save(tx)
		if err != nil {
			return err
		}
	}

	if a.Tag.ID == a.To.ID {
		return errors.New("can't assign alias onto itself")
	}

	if err = updatePtm(tx, a.To, a.Tag); err != nil {
		return err
	}

	if err != nil {
		log.Println(err)
		log.Println(a.To, a.Tag)
		return err
	}

	if err = updateAliases(tx, a.To, a.Tag); err != nil {
		return err
	}

	if err = updateParents(tx, a.To, a.Tag); err != nil {
		return err
	}

	_, err = tx.Exec(`
			INSERT INTO alias(
				alias_from,
				alias_to
			)
			VALUES ($1, $2)
		`,
		a.Tag.ID,
		a.To.ID,
	)

	resetCacheTag(a.To.QID(tx))
	resetCacheTag(a.Tag.QID(tx))

	return err
}

func updatePtm(q querier, to, from *Tag) error {
	// Is to, from aliased to something else
	// If so return error
	ato, err := to.aliasedTo(q)
	if err != nil {
		return err
	}

	afr, err := from.aliasedTo(q)
	if err != nil {
		return err
	}

	if ato.ID != to.QID(q) || from.QID(q) != afr.ID {
		return errors.New("assign on the aliased tag")
	}

	// Gather all parents of to
	var tags = []*Tag{to}
	tags = append(tags, to.allParents(q)...)

	// Assign all above tags to from
	for _, tag := range tags {
		_, err := q.Exec(`
			INSERT INTO post_tag_mappings(
				post_id, tag_id
			)
			SELECT post_id, $1
			FROM post_tag_mappings
			WHERE tag_id = $2
			ON CONFLICT DO NOTHING
			`,
			tag.ID,
			from.ID,
		)
		if err != nil {
			return err
		}
	}

	// Remove all from mappings
	_, err = q.Exec(`
		DELETE FROM post_tag_mappings
		WHERE tag_id = $1
		`,
		from.ID,
	)
	if err != nil {
		return err
	}

	return nil
}

func updateAliases(q querier, to, from *Tag) error {
	// Update aliases
	_, err := q.Exec(`
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

func updateParents(q querier, to, from *Tag) error {
	_, err := q.Exec(`
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
	_, err = q.Exec(`
		DELETE FROM parent_tags
		WHERE child_id = $1
		`,
		from.ID,
	)
	if err != nil {
		return err
	}

	_, err = q.Exec(`
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

	_, err = q.Exec(`
		DELETE FROM parent_tags
		WHERE parent_id = $1
		`,
		from.ID,
	)

	return err
}
