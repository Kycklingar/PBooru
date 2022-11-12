package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/kycklingar/PBooru/DataManager/namespace"
	"github.com/kycklingar/PBooru/DataManager/query"
	"github.com/kycklingar/PBooru/DataManager/sqlbinder"
	"github.com/kycklingar/set"
	"github.com/kycklingar/sqhell/cond"
)

func TagFromID(id int) (Tag, error) {
	var tag Tag
	return tag, DB.QueryRow(`
		SELECT id, tag, namespace, count
		FROM tag
		WHERE id = $1
		`,
		id,
	).Scan(&tag.ID, &tag.Tag, &tag.Namespace, &tag.Count)
}

type Namespace = namespace.Namespace

type Tag struct {
	ID        int
	Tag       string
	Namespace Namespace
	Count     int
}

const (
	FID sqlbinder.Field = iota
	FTag
	FCount
	FNamespace
)

func (t *Tag) Rebind(sel *sqlbinder.Selection, field sqlbinder.Field) {
	switch field {
	case FID:
		sel.Rebind(&t.ID)
	case FTag:
		sel.Rebind(&t.Tag)
	case FCount:
		sel.Rebind(&t.Count)
	case FNamespace:
		sel.Rebind(&t.Namespace)
	}
}

func (t Tag) String() string {
	if t.Namespace == "none" {
		return t.Tag
	}

	return fmt.Sprint(t.Namespace, ":", t.Tag)
}

func (t Tag) EditString() string {
	if t.Namespace == "none" && strings.HasPrefix(t.Tag, ":") {
		return ":" + t.Tag
	}
	return t.String()
}

func (t Tag) Escaped() string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			t.EditString(),
			"\\",
			"\\\\",
		),
		",",
		"\\,",
	)
}

func (t *Tag) parse(str string) {
	split := strings.SplitN(str, ":", 2)

	switch len(split) {
	case 1:
		t.Tag = strings.TrimSpace(split[0])
	case 2:
		t.Namespace = Namespace(strings.TrimSpace(split[0]))
		t.Tag = strings.TrimSpace(split[1])
	}

	if t.Namespace == "" {
		t.Namespace = "none"
	}
}

// Get the aliasing of the tag
// returns aliased_to, aliased_from, error
func (t Tag) Aliasing() (to *Tag, from []Tag, err error) {
	var tmp Tag

	// aliased to
	err = DB.QueryRow(`
		SELECT id, tag, namespace
		FROM tag
		JOIN alias
		ON id = alias_to
		WHERE alias_from = $1
		`,
		t.ID,
	).Scan(&tmp.ID, &tmp.Tag, &tmp.Namespace)
	switch err {
	case nil:
		to = &tmp
	case sql.ErrNoRows:
	default:
		return
	}

	// aliased from
	err = query.Rows(
		DB,
		`SELECT id, tag, namespace
		FROM tag
		JOIN alias
		ON id = alias_from
		WHERE alias_to = $1`,
		t.ID,
	)(func(scan scanner) error {
		var t Tag
		err := scan(&t.ID, &t.Tag, &t.Namespace)
		from = append(from, t)
		return err
	})

	return
}

// Get the family tree of the tag
// returns children, parents, grandchildren, grandparents, error
func (t Tag) Family() (children, parents, grandChildren, grandParents []Tag, err error) {
	// children
	err = query.Rows(
		DB,
		`SELECT id, tag, namespace
		FROM tag
		LEFT JOIN parent_tags
		ON id = child_id
		WHERE parent_id = $1
		ORDER BY count DESC`,
		t.ID,
	)(func(scan scanner) error {
		var child Tag
		err := scan(&child.ID, &child.Tag, &child.Namespace)
		children = append(children, child)
		return err
	})
	if err != nil {
		return
	}

	// parents
	err = query.Rows(
		DB,
		`SELECT id, tag, namespace
		FROM tag
		RIGHT JOIN parent_tags
		ON id = parent_id
		WHERE child_id = $1
		ORDER BY count DESC`,
		t.ID,
	)(func(scan scanner) error {
		var parent Tag
		err := scan(&parent.ID, &parent.Tag, &parent.Namespace)
		parents = append(parents, parent)
		return err
	})
	if err != nil {
		return
	}

	//TODO grandparents, grandchildren
	pchain := tagChain(parents)
	gchain := pchain.copy().parents(DB)
	grandParents = set.Diff(gchain.set, pchain.set)
	err = gchain.err

	return
}

func (t *Tag) save(q querier) error {
	if t.ID != 0 {
		return nil
	}

	if t.Tag == "" {
		return errors.New(fmt.Sprintf("tag: not enough arguments. Expected Tag got '%s'", t.Tag))
	}

	err := q.QueryRow(`
		SELECT id
		FROM tag
		WHERE tag = $1
		AND namespace = $2
		`,
		t.Tag,
		t.Namespace,
	).Scan(&t.ID)
	switch err {
	case nil:
		return nil
	default:
		return err
	case sql.ErrNoRows:
	}

	nid, err := t.Namespace.Create(q)
	if err != nil {
		return err
	}

	err = q.QueryRow(`
		INSERT INTO tags(tag, namespace_id)
		VALUES($1, $2)
		RETURNING id
		`,
		t.Tag,
		nid,
	).Scan(&t.ID)
	if err != nil {
		return err
	}

	return nil
}

func (t *Tag) recount(q querier) error {
	_, err := q.Exec(`
		UPDATE tags
		SET count = (
			SELECT count(1)
			FROM post_tag_mappings
			WHERE tag_id = $1
		)
		WHERE id = $1
		`,
		t.ID,
	)
	return err
}

func (t *Tag) updateCount(q querier, count int) error {
	_, err := q.Exec(`
		UPDATE tags
		SET count = count + $1
		WHERE id = $2
		`,
		count,
		t.ID,
	)
	return err
}

type TagsResult struct {
	Tags  []Tag
	Count int
}

func SearchTags(tagstr string, limit, offset int) (TagsResult, error) {
	var (
		result      TagsResult
		values      []any
		count       = "SELECT count(*)\n"
		sel         = "SELECT id, tag, namespace, count\n"
		from        = "FROM tag\n"
		where       = cond.NewGroup().Add("", cond.N("WHERE count > 0"))
		order       = "ORDER BY id ASC\n"
		limitoffset cond.Group
		cursor      = 1
	)

	set := parseTags(tagstr)
	if len(set) > 0 {
		t := set[0]
		where.Add("AND", cond.P("tag LIKE('%%'||$%d||'%%')\n"))
		values = append(values, strings.ReplaceAll(t.Tag, "\\", "\\\\"))

		if t.Namespace != "none" {
			where.Add("AND", cond.P("namespace = $%d\n"))
			values = append(values, t.Namespace)
		}
		order = "ORDER BY count DESC, id ASC\n"
	}

	err := DB.QueryRow(fmt.Sprint(count, from, where.Eval(&cursor)), values...).Scan(&result.Count)
	if err != nil {
		return result, err
	}

	// reset cursor
	cursor = 1

	limitoffset.Add("", cond.P("LIMIT $%d")).Add("", cond.P("OFFSET $%d"))
	values = append(values, limit, offset)

	return result, query.Rows(
		DB,
		fmt.Sprint(sel, from, where.Eval(&cursor), order, limitoffset.Eval(&cursor)),
		values...,
	)(func(scan scanner) error {
		var t Tag
		err := scan(&t.ID, &t.Tag, &t.Namespace, &t.Count)
		result.Tags = append(result.Tags, t)
		return err
	})
}

func TagHints(str string) ([]Tag, error) {
	var res []Tag
	tags := parseTags(str)
	for _, tag := range tags {
		var (
			nwhere, njoin string
			values        = []any{strings.ReplaceAll(tag.Tag, "\\", "\\\\")}
		)
		if tag.Namespace != "none" {
			njoin = `
				JOIN namespaces n
				ON namespace_id = n.id
				`
			nwhere = "AND nspace = $2"
			values = append(values, string(tag.Namespace))
		}

		err := query.Rows(
			DB,
			fmt.Sprintf(`
				SELECT tag, namespace, count
				FROM tag
				WHERE id IN (
					SELECT COALESCE(alias_to, tags.id)
					FROM tags
					%s
					LEFT JOIN alias
					ON tags.id = alias_from
					WHERE tag LIKE('%%'||$1||'%%')
					%s
				)
				ORDER BY count DESC
				LIMIT 10`,
				njoin,
				nwhere,
			),
			values...,
		)(func(scan scanner) error {
			var t Tag
			err := scan(&t.Tag, &t.Namespace, &t.Count)
			res = append(res, t)
			return err
		})
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}
