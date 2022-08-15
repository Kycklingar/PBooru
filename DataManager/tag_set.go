package DataManager

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/kycklingar/set"
)

type tsTag Tag

func (a tsTag) Less(b tsTag) bool {
	return Tag(a).String() < Tag(b).String()
}

func (a Tag) Less(b Tag) bool {
	return a.ID < b.ID
}

func tSetStr(s set.Sorted[Tag]) string {
	return sep(",", len(s), func(i int) string {
		return strconv.Itoa(s[i].ID)
	})
}

func tsTags(set set.Sorted[tsTag]) []Tag {
	var tags = make([]Tag, len(set))
	for i, tag := range set {
		tags[i] = Tag(tag)
	}

	return tags
}

type tsSetChain struct {
	set set.Sorted[tsTag]
	err error
}

func (chain tsSetChain) unwrap() (set.Sorted[tsTag], error) {
	return chain.set, chain.err
}

func tsChain(set set.Sorted[tsTag]) tsSetChain {
	return tsSetChain{
		set: set,
	}
}

func (old tsSetChain) qids(q querier) tagSetChain {
	var chain = tagSetChain{
		err: old.err,
	}

	if chain.err != nil {
		return chain
	}

	var stmt *sql.Stmt

	stmt, chain.err = q.Prepare(`
		SELECT id
		FROM tag t
		WHERE tag = $1
		AND namespace = $2
		`,
	)
	if chain.err != nil {
		return chain
	}
	defer stmt.Close()

	for _, t := range old.set {
		chain.err = stmt.QueryRow(t.Tag, t.Namespace).Scan(&t.ID)
		if chain.err != nil {
			return chain
		}
		chain.set.Set(Tag(t))
	}

	return chain
}

func (old tsSetChain) save(q querier) tagSetChain {
	var chain = tagSetChain{
		err: old.err,
	}
	if chain.err != nil {
		return chain
	}

	for _, t := range old.set {
		var tag = Tag(t)
		if chain.err = tag.save(q); chain.err != nil {
			return chain
		}

		chain.set.Set(tag)
	}

	return chain
}

func parseTagsWithID(q querier, tagstr string) (set.Sorted[Tag], error) {
	return tsChain(parseTags(tagstr)).qids(q).unwrap()
}

func parseTags(tagStr string) set.Sorted[tsTag] {
	var (
		tagSpitter = make(chan string)
		set        = set.New[tsTag]()
	)

	go func(spitter chan string) {
		const (
			next = iota
			unescape
		)

		var (
			state = next
			tag   strings.Builder
		)

		var spit = func() {
			spitter <- strings.ToLower(strings.TrimSpace(tag.String()))
		}

		for _, c := range tagStr {
			switch state {
			case next:
				switch c {
				case '\\':
					state = unescape
				case ',', '\n':
					spit()
					tag.Reset()
				default:
					tag.WriteRune(c)
				}
			case unescape:
				switch c {
				case '\n': //ignore newlines
				default:
					tag.WriteRune(c)
				}
				state = next
			}
		}

		if tag.Len() > 0 {
			spit()
		}

		close(spitter)
	}(tagSpitter)

	for tag := range tagSpitter {
		var t Tag
		t.parse(tag)

		// Discard empty tags
		if t.Tag == "" {
			continue
		}

		set.Set(tsTag(t))
	}

	return set
}
