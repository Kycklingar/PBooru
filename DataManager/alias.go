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

func (a *Alias) QTo(q querier) *Tag {
	if a.Tag.QID(q) == 0 {
		return a.To
	}
	if a.To.QID(q) != 0 {
		return a.To
	}

	err := q.QueryRow("SELECT alias_to FROM alias WHERE alias_from=$1", a.Tag.QID(q)).Scan(&a.To.ID)
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

	_, err := q.Exec("INSERT INTO alias (alias_from, alias_to) VALUES($1, COALESCE((SELECT alias_to FROM alias WHERE alias_from = $2), $2)) ON CONFLICT (alias_from) DO UPDATE SET alias_to = EXCLUDED.alias_to", a.Tag.QID(q), a.To.QID(q))
	if err != nil {
		log.Println(err)
		log.Println(a.To, a.Tag)
	}
	resetCacheTag(a.To.QID(q))
	resetCacheTag(a.Tag.QID(q))

	return err
}
