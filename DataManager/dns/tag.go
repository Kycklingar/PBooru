package dns

import "database/sql"

type Tag struct {
	Id          string
	Name        string
	Description string
	Score       int
}

func AllTags(db *sql.DB) ([]Tag, error) {
	rows, err := db.Query(`
		SELECT id, name, description, score
		FROM dns_tag
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag

	for rows.Next() {
		var tag Tag
		err = rows.Scan(
			&tag.Id,
			&tag.Name,
			&tag.Description,
			&tag.Score,
		)
		if err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func CreateTag(db *sql.DB, id, name, descr string, score int) error {
	_, err := db.Exec(`
		INSERT INTO dns_tag (
			id,
			name,
			description,
			score
		)
		VALUES ($1, $2, $3, $4)
		`,
		id, name, descr, score,
	)

	return err
}

func UpdateTag(db *sql.DB, id, name, descr string, score int) error {
	_, err := db.Exec(`
		UPDATE dns_tag
		SET name = $1,
		description = $2,
		score = $3
		WHERE id = $4
		`,
		name,
		descr,
		score,
		id,
	)

	return err
}
