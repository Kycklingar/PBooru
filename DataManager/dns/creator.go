package dns

import "database/sql"

type Creator struct {
	Name string
}

func NewCreator(db *sql.DB, creator Creator) error {
	_, err := db.Exec(`
		INSERT INTO dns_creator(name)
		VALUES($1)
		`,
		creator.Name,
	)

	return err
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
