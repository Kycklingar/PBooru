package DataManager

import (
	"database/sql"
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
	rows, err := q.Query("SELECT alias_from FROM alias WHERE alias_to=?", a.Tag.QID(q))
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

func (a *Alias) QTo(q querier) *Tag {
	if a.Tag.QID(q) == 0 {
		return a.To
	}
	if a.To.QID(q) != 0 {
		return a.To
	}

	err := q.QueryRow("SELECT alias_to FROM alias WHERE alias_from=?", a.Tag.QID(q)).Scan(&a.To.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
	}

	return a.To
}

func (a *Alias) Save(q querier) error {
	if a.Tag.QID(q) == 0 {
		err := a.Tag.Save(q)
		if err != nil {
			return err
		}
	}
	if a.To.QID(q) == 0 {
		err := a.To.Save(q)
		if err != nil {
			return err
		}
	}

	// Check if to have a relation and append last
	b := NewAlias()
	b.Tag = a.To

	for {
		if b.QTo(q).QID(q) == 0 {
			a.To = b.Tag
			break
		}
		b.Tag = b.To
		b.To = NewTag()
	}

	// Check if from have a relation and change them to To
	for _, from := range a.QFrom(q) {
		b := NewAlias()
		b.Tag = from

		b.To = a.To
		err := b.Save(q)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	// Find all parent/child relations and act on them
	if _, err := q.Exec("UPDATE OR REPLACE parent_tags SET parent_id=? WHERE parent_id=?", a.To.QID(q), a.Tag.QID(q)); err != nil {
		log.Println(err)
		return err
	}

	if _, err := q.Exec("UPDATE OR REPLACE parent_tags SET child_id=? WHERE child_id=?", a.To.QID(q), a.Tag.QID(q)); err != nil {
		log.Println(err)
		return err
	}

	if _, err := q.Exec("INSERT OR IGNORE INTO post_tag_mappings(post_id, tag_id) SELECT post_id, (SELECT parent_id FROM parent_tags WHERE child_id=?) FROM post_tag_mappings WHERE tag_id=?", a.To.QID(q), a.Tag.QID(q)); err != nil {
		log.Println(err)
		return err
	}

	// Finnaly insert/update post_tag_mappings with new alias
	if _, err := q.Exec("UPDATE OR REPLACE post_tag_mappings SET tag_id=? WHERE tag_id=?", a.To.QID(q), a.Tag.QID(q)); err != nil {
		log.Println(err)
		return err
	}

	_, err := q.Exec("INSERT OR IGNORE INTO alias (alias_from, alias_to) VALUES(?, ?)", a.Tag.QID(q), a.To.QID(q))

	resetCacheTag(a.To.QID(q))
	resetCacheTag(a.Tag.QID(q))

	return err
}
