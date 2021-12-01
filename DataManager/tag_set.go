package DataManager

import "strings"

type tagSet []*Tag

// returns the tags in a that are not in b
func tagSetDiff(a, b tagSet) tagSet {
	var (
		diff = make(map[string]struct{})
		ret  []*Tag
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

		parents = append(parents, tagSetDiff(par, parents)...)
	}

	return append(set, tagSetDiff(parents, set)...), nil
}

func (set tagSet) save(q querier) error {
	for _, tag := range set {
		if err := tag.Save(q); err != nil {
			return err
		}
	}

	return nil
}
