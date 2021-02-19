package DataManager

import "log"

func (p *Post) SetAlt(q querier, altof int) error {
	log.Println(p.ID, altof)
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
	_, err := q.Exec(`
		UPDATE posts
		SET alt_group = id
		WHERE id = $1
		`,
		p.ID,
	)

	return err
}
