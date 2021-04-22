package DataManager

import (
	"strconv"

	c "github.com/kycklingar/PBooru/DataManager/cache"
)

func (p *Post) SetAlt(q querier, altof int) error {
	_, err := q.Exec(`
		WITH d1 AS (
			SELECT * FROM get_dupe($1)
		),
		d2 AS (
			SELECT * FROM get_dupe($2)
		),
		altgroups AS (
			SELECT *
			FROM posts
			WHERE alt_group IN(
				SELECT alt_group
				FROM posts
				WHERE id IN(
					(SELECT * FROM d1),
					(SELECT * FROM d2)
				)
			)
		)

		UPDATE posts
		SET alt_group = (
			SELECT MAX(id)
			FROM altgroups
		)
		WHERE id IN(
			SELECT id
			FROM altgroups
		)
		`,
		p.ID,
		altof,
	)

	tc := new(TagCollector)
	tc.GetFromPost(q, p)
	for _, tag := range tc.Tags {
		resetCacheTag(q, tag.ID)
	}

	c.Cache.Purge("TPC", strconv.Itoa(p.ID))

	return err
}

func (p *Post) RemoveAlt(q querier) error {
	// Reassign alt_group id's away from the removed
	_, err := q.Exec(`
		UPDATE posts
		SET alt_group = (
			SELECT MAX(id)
			FROM posts
			WHERE alt_group = $1
			AND id != $1
		) WHERE alt_group = $1
		`,
		p.ID,
	)
	if err != nil {
		return err
	}

	// Reset to default
	_, err = q.Exec(`
		UPDATE posts
		SET alt_group = id
		WHERE id = $1
		`,
		p.ID,
	)

	tc := new(TagCollector)
	tc.GetFromPost(q, p)
	for _, tag := range tc.Tags {
		resetCacheTag(q, tag.ID)
	}

	c.Cache.Purge("TPC", strconv.Itoa(p.ID))

	return err
}
