package DataManager

func (p *Post) SetAlt(altof int, q querier) error {
	_, err := q.Exec(`
		UPDATE posts
		SET alt_group = $1
		WHERE id = $2
		`,
		altof,
		p.ID,
	)

	return err
}

func (p *Post) RemoveAlt(q querier) error {
	_, err := q.Exec(`
		UPDATE posts
		SET alt_group = id
		WHERE id = $1
		`,
		p.ID,
	)

	return err
}
