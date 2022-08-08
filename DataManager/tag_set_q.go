package DataManager

import (
	"database/sql"
	"fmt"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

type tagSetChain struct {
	set tagSet
	err error
}

func (set tagSet) chain() tagSetChain {
	return tagSetChain{set: set}
}

func (chain tagSetChain) unwrap() (tagSet, error) {
	return chain.set, chain.err
}

func (chain tagSetChain) qids(q querier) tagSetChain {
	if chain.err != nil {
		return chain
	}

	var stmt *sql.Stmt

	stmt, chain.err = q.Prepare(`
		SELECT t.id
		FROM tags t
		JOIN namespaces n
		ON t.namespace_id = n.id
		WHERE t.tag = $1
		AND n.nspace = $2
		`,
	)
	if chain.err != nil {
		return chain
	}
	defer stmt.Close()

	for _, t := range chain.set {
		chain.err = stmt.QueryRow(t.Tag, t.Namespace.Namespace).Scan(&t.ID)
		if chain.err != nil {
			return chain
		}
	}

	return chain
}

func postTags(q querier, postID int) tagSetChain {
	var (
		chain tagSetChain
		rows  *sql.Rows
	)

	rows, chain.err = q.Query(`
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
	if chain.err != nil {
		return chain
	}
	defer rows.Close()

	for rows.Next() {
		var t = NewTag()

		chain.err = rows.Scan(
			&t.ID,
			&t.Tag,
			&t.Namespace.ID,
			&t.Namespace.Namespace,
		)
		if chain.err != nil {
			return chain
		}

		chain.set = append(chain.set, t)
	}

	return chain
}

// upgrade will replace aliases and add parent tags to the set
func (chain tagSetChain) upgrade(q querier) tagSetChain {
	if chain.err != nil {
		return chain
	}

	var parents tagSet

	chain = chain.aliases(q)
	if chain.err != nil {
		return chain
	}

	for _, t := range chain.set {
		var par tagSet

		par, chain.err = t.parents(q)
		if chain.err != nil {
			return chain
		}

		parents = append(parents, par.diff(parents)...)
	}

	chain.set = append(chain.set, parents.diff(chain.set)...)
	return chain
}

func (chain tagSetChain) aliases(q querier) tagSetChain {
	if chain.err != nil {
		return chain
	}

	for i := range chain.set {
		chain.set[i], chain.err = aliasedTo(q, chain.set[i])
		if chain.err != nil {
			return chain
		}
	}

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

	for _, tag := range chain.set {
		_, chain.err = stmt.Exec(n, tag.ID)
		if chain.err != nil {
			return chain
		}
	}

	return chain
}

func (chain tagSetChain) recount(q querier) tagSetChain {
	if chain.err != nil || len(chain.set) <= 0 {
		return chain
	}

	_, chain.err = q.Exec(
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
			sep(",", len(chain.set), chain.set.strindex),
		),
	)

	return chain
}

func (chain tagSetChain) purgeCountCache(q querier) tagSetChain {
	if chain.err != nil || len(chain.set) <= 0 {
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
			sep(",", len(chain.set), chain.set.strindex),
		),
	)

	// Legacy
	for _, t := range chain.set {
		C.Cache.Purge("PC", strconv.Itoa(t.ID))
	}
	C.Cache.Purge("PC", "0")

	return chain
}

func (chain tagSetChain) save(q querier) tagSetChain {
	if chain.err != nil {
		return chain
	}

	for _, tag := range chain.set {
		if chain.err = tag.Save(q); chain.err != nil {
			return chain
		}
	}

	return chain
}
