package DataManager

func (p *Post) SetAlt(q querier, altof int) error {
	_, err := q.Exec(`
		UPDATE posts
		SET alt_group = (
			SELECT alt_group
			FROM posts
			WHERE id = $1
		)
		WHERE alt_group = (
			SELECT alt_group
			FROM posts
			WHERE id = $2
		)
		`,
		altof,
		p.ID,
	)

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

	return err
}
