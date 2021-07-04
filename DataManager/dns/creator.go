package dns

import "database/sql"

type Creator struct {
	Name string
}

func NewCreator(db *sql.DB, name string) (int, error) {
	var cid int

	err := db.QueryRow(`
		INSERT INTO dns_creator(name)
		VALUES($1)
		RETURNING id
		`,
		name,
	).Scan(&cid)

	return cid, err
}

func CreatorEditName(db *sql.DB, creatorID int, name string) error {
	_, err := db.Exec(`
		UPDATE dns_creator
		SET name = $1
		WHERE id = $2
		`,
		name,
		creatorID,
	)

	return err
}

func EditTags(db *sql.DB, creatorID int, enabledTags []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete all current tags
	_, err = tx.Exec(`
		DELETE FROM dns_tags
		WHERE creator_id = $1
		`,
		creatorID,
	)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO dns_tags(creator_id, tag_id)
		VALUES($1, $2)
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range enabledTags {
		if _, err = stmt.Exec(creatorID, tag); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func AddUrl(db *sql.DB, creatorId int, url, domain string) error {
	_, err := db.Exec(`
		INSERT INTO dns_creator_urls(id, domain, url)
		VALUES(
			$1,
			(
				SELECT id
				FROM dns_domain
				WHERE domain = $2
			),
			$3
		)
		`,
		creatorId,
		domain,
		url,
	)

	return err
}

func RemoveUrl(db *sql.DB, creatorId int, url string) error {
	_, err := db.Exec(`
		DELETE FROM dns_creator_urls
		WHERE id = $1
		AND url = $2
		`,
		creatorId,
		url,
	)

	return err
}

func MapTag(db *sql.DB, creatorID, tagID int) error {
	_, err := db.Exec(`
		INSERT INTO dns_tag_mapping(tag_id, creator_id)
		VALUES($1, $2)
		`,
		tagID,
		creatorID,
	)

	return err
}
