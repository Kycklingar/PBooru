package dns

import "database/sql"

func DomainNew(db *sql.DB, domain string) error {
	_, err := db.Exec(`
		INSERT INTO dns_domain(domain)
		VALUES($1)
		`,
		domain,
	)

	return err
}
