package DataManager

import (
	"strconv"

	c "github.com/kycklingar/PBooru/DataManager/cache"
)

func (p *Post) SetAlt(q querier, altof int) error {
	d, err := getDupeFromPost(q, p)
	if err != nil {
		return err
	}

	p2 := NewPost()
	p2.ID = altof
	d2, err := getDupeFromPost(q, p2)
	if err != nil {
		return err
	}

	_, err = q.Exec(`
		UPDATE posts
		SET alt_group = (
			SELECT alt_group
			FROM posts
			WHERE id = $1
		)
		WHERE id IN (
			SELECT id
			FROM posts
			LEFT JOIN duplicates d
			ON id = d.dup_id
			WHERE d.dup_id IS NULL
			AND alt_group = (
				SELECT alt_group
				FROM posts
				WHERE id = $2
			)
		)
		`,
		d2.Post.ID,
		d.Post.ID,
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
			SELECT id
			FROM posts
			WHERE alt_group = $1
			AND id != $1
			LIMIT 1
		) WHERE alt_group = $1
		`,
		p.ID,
	)
	if err != nil {
		return err
	}

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
