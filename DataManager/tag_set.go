package DataManager

import (
	"fmt"
	"strconv"
	"strings"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

type tagSet []*Tag

func (set tagSet) strindex(i int) string {
	return strconv.Itoa(set[i].ID)
}

// returns the tags in a that are not in b
func (a tagSet) diff(b tagSet) tagSet {
	var (
		diff = make(map[string]struct{})
		ret  tagSet
	)

	for _, tag := range b {
		diff[tag.String()] = struct{}{}
	}

	for _, tag := range a {
		if _, ok := diff[tag.String()]; !ok {
			ret = append(ret, tag)
		}
	}

	return ret
}

func (a tagSet) unique() tagSet {
	m := make(map[string]*Tag)

	for _, t := range a {
		m[t.String()] = t
	}

	var (
		retu = make(tagSet, len(m))
		i    int
	)

	for _, t := range m {
		retu[i] = t
		i++
	}

	return retu
}

func parseTags(tagstr string, delim rune) (tagSet, error) {
	var (
		tagSpitter = make(chan string)
		set        tagSet
	)

	go func(spitter chan string) {
		const (
			next = iota
			unescape
		)

		var (
			state = next
			tag   string
		)

		for _, c := range tagstr {
			switch state {
			case next:
				switch c {
				case '\\':
					state = unescape
				case delim:
					spitter <- strings.TrimSpace(tag)
					tag = ""
				default:
					tag += string(c)
				}
			case unescape:
				tag += string(c)
				state = next
			}
		}

		spitter <- tag
		close(spitter)
	}(tagSpitter)

	for tag := range tagSpitter {
		if tag == "" {
			continue
		}

		t := NewTag()
		if err := t.Parse(tag); err != nil {
			return nil, err
		}

		set = append(set, t)
	}

	return set, nil
}

func postTags(q querier, postID int) (tagSet, error) {
	var set tagSet

	rows, err := q.Query(`
		SELECT t.id, t.tag, n.id, n.nspace
		FROM post_tag_mappings
		JOIN tags t
		ON tag_id = t.id
		JOIN namespaces n
		ON t.namespace_id = n.id
		WHERE post_id = $1
		`,
		postID,
	)
	if err != nil {
		return set, noRows(err)
	}
	defer rows.Close()

	for rows.Next() {
		var t = NewTag()

		err = rows.Scan(
			&t.ID,
			&t.Tag,
			&t.Namespace.ID,
			&t.Namespace.Namespace,
		)
		if err != nil {
			return set, err
		}

		set = append(set, t)
	}

	return set, nil
}

// upgrade will replace aliases and add parent tags to the set
func (set tagSet) upgrade(q querier) (tagSet, error) {
	var (
		err     error
		parents tagSet
	)

	for i := range set {
		set[i], err = aliasedTo(q, set[i])
		if err != nil {
			return set, err
		}

		par, err := set[i].parents(q)
		if err != nil {
			return set, err
		}

		parents = append(parents, par.diff(parents)...)
	}

	return append(set, parents.diff(set)...), nil
}

func (set tagSet) recount(q querier) error {
	if len(set) <= 0 {
		return nil
	}

	_, err := q.Exec(
		fmt.Sprintf(`
			WITH tag_counts AS (
				SELECT tag_id, count(*)
				FROM post_tag_mappings
				WHERE tag_id IN(%s)
				GROUP BY tag_id
			)
			UPDATE tags
			SET count = c.count
			FROM tag_counts c
			WHERE c.tag_id = id
			`,
			sep(",", len(set), set.strindex),
		),
	)

	return err
}

func (set tagSet) purgeCountCache(q querier) error {
	if len(set) <= 0 {
		return nil
	}

	_, err := q.Exec(
		fmt.Sprintf(`
			DELETE FROM search_count_cache
			WHERE id IN(
				SELECT cache_id
				FROM search_count_cache_tag_mapping
				WHERE tag_id IN(%s)
			)
			`,
			sep(",", len(set), set.strindex),
		),
	)

	// Legacy
	for _, t := range set {
		C.Cache.Purge("PC", strconv.Itoa(t.ID))
	}
	C.Cache.Purge("PC", "0")

	return err
}

func (set tagSet) save(q querier) error {
	for _, tag := range set {
		if err := tag.Save(q); err != nil {
			return err
		}
	}

	return nil
}
