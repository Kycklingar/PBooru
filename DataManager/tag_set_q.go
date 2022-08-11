package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/set"
)

type tagSetChain struct {
	set set.Sorted[Tag]
	err error
}

func tagChain(from any) tagSetChain {
	var chain tagSetChain
	switch t := from.(type) {
	case Tag:
		chain.set = set.New[Tag](lessfnTagID, t)
	case []Tag:
		chain.set = set.New[Tag](lessfnTagID, t...)
	case set.Sorted[Tag]:
		chain.set = t
	default:
		panic(errors.New("non tag type"))
	}

	return chain
}

func (chain tagSetChain) less(f func(Tag, Tag) bool) tagSetChain {
	var newSet = set.New[Tag](f)
	newSet.Set(chain.set.Slice...)
	chain.set = newSet
	return chain
}

func (chain tagSetChain) unwrap() (set.Sorted[Tag], error) {
	return chain.set, chain.err
}

func (chain tagSetChain) qids(q querier) tagSetChain {
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

	var newSet = set.New[Tag](lessfnTagID)

	for _, t := range chain.set.Slice {
		chain.err = stmt.QueryRow(t.Tag, t.Namespace).Scan(&t.ID)
		if chain.err != nil {
			return chain
		}
		newSet.Set(t)
	}

	chain.set = newSet
	return chain
}

func (chain tagSetChain) query(q querier) tagSetChain {
	if chain.err != nil || len(chain.set.Slice) <= 0 {
		return chain
	}

	chain.err = query(
		q,
		fmt.Sprintf(
			`SELECT id, tag, namespace
			FROM tag
			WHERE id IN(%s)`,
			tSetStr(chain.set),
		),
	)(func(scan scanner) error {
		var t Tag
		err := scan(&t.ID, &t.Tag, &t.Namespace)
		chain.set.Set(t)
		return err
	})

	return chain
}

func postTags(q querier, postID int) tagSetChain {
	var chain = tagSetChain{
		set: set.New[Tag](lessfnTag),
	}

	chain.err = query(
		q,
		`SELECT id, tag, namespace, count
		FROM post_tag_mappings
		JOIN tag
		ON tag_id = id
		WHERE post_id = $1`,
		postID,
	)(func(scan scanner) error {
		var t Tag

		err := scan(
			&t.ID,
			&t.Tag,
			&t.Namespace,
			&t.Count,
		)

		chain.set.Set(t)
		return err
	})

	return chain
}

// upgrade will replace aliases and add parent tags to the set
func (chain tagSetChain) upgrade(q querier) tagSetChain {
	return chain.aliases(q).parents(q)
}

// add the parents and grand parents to the set
func (chain tagSetChain) parents(q querier) tagSetChain {
	var (
		// only query a tag once
		queriedTags = set.New[Tag](lessfnTagID)
		toBeQueried = chain.set
	)

	for len(toBeQueried.Slice) > 0 {
		queriedTags = set.Union(queriedTags, toBeQueried)

		var queryStr = fmt.Sprintf(
			`SELECT id, tag, namespace
			FROM tag
			JOIN parent_tags
			ON parent_id = id
			WHERE child_id IN(%s)`,
			tSetStr(toBeQueried),
		)

		toBeQueried = set.New[Tag](lessfnTagID)

		chain.err = query(q, queryStr)(func(scan scanner) error {
			var t Tag
			err := scan(&t.ID, &t.Tag, &t.Namespace)
			if err != nil {
				return err
			}

			chain.set.Set(t)
			if !queriedTags.Has(t) {
				toBeQueried.Set(t)
			}
			return nil
		})
		if chain.err != nil {
			break
		}
	}

	return chain
}

// TODO query all aliases
func (chain tagSetChain) aliases(q querier) tagSetChain {
	if chain.err != nil {
		return chain
	}

	var aliased = set.New[Tag](lessfnTagID)

	for _, tag := range chain.set.Slice {
		var a Tag
		a, chain.err = aliasedTo(q, tag)
		if chain.err != nil {
			return chain
		}
		aliased.Set(a)
	}

	chain.set = aliased

	return chain
}

func (chain tagSetChain) addCount(q querier, n int) tagSetChain {
	if chain.err != nil {
		return chain
	}

	var stmt *sql.Stmt
	stmt, chain.err = q.Prepare(`
		UPDATE tags
		SET count = count + $1
		WHERE id = $2
		`,
	)
	if chain.err != nil {
		return chain
	}

	defer stmt.Close()

	for _, tag := range chain.set.Slice {
		_, chain.err = stmt.Exec(n, tag.ID)
		if chain.err != nil {
			return chain
		}
	}

	return chain
}

func (chain tagSetChain) recount(q querier) tagSetChain {
	if chain.err != nil || len(chain.set.Slice) <= 0 {
		return chain
	}

	_, chain.err = q.Exec(
		fmt.Sprintf(`
			WITH tag_counts AS (
				SELECT id, count(tag_id)
				FROM tags
				LEFT JOIN post_tag_mappings
				ON id = tag_id
				WHERE id IN(%s)
				GROUP BY id
			)
			UPDATE tags
			SET count = c.count
			FROM tag_counts c
			WHERE c.id = tags.id
			`,
			tSetStr(chain.set),
		),
	)

	return chain
}

func (chain tagSetChain) purgeCountCache(q querier) tagSetChain {
	if chain.err != nil || len(chain.set.Slice) <= 0 {
		return chain
	}

	_, chain.err = q.Exec(
		fmt.Sprintf(`
			DELETE FROM search_count_cache
			WHERE id IN(
				SELECT cache_id
				FROM search_count_cache_tag_mapping
				WHERE tag_id IN(%s)
			)
			`,
			tSetStr(chain.set),
		),
	)

	// Legacy
	for _, t := range chain.set.Slice {
		C.Cache.Purge("PC", strconv.Itoa(t.ID))
	}
	C.Cache.Purge("PC", "0")

	return chain
}

func (chain tagSetChain) save(q querier) tagSetChain {
	if chain.err != nil {
		return chain
	}

	for i := range chain.set.Slice {
		if chain.err = chain.set.Slice[i].save(q); chain.err != nil {
			return chain
		}
	}

	return chain
}
