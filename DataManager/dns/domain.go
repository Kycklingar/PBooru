package dns

import (
	"database/sql"
	"fmt"

	"github.com/kycklingar/sqhell/cond"
)

type Domain struct {
	Id     int
	Domain string
	Icon   string
}

type URL string

func Domains(db *sql.DB) ([]Domain, error) {
	rows, err := db.Query(`
		SELECT id, domain, icon
		FROM dns_domain
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain

	for rows.Next() {
		var dom Domain

		err = rows.Scan(&dom.Id, &dom.Domain, &dom.Icon)
		if err != nil {
			return nil, err
		}

		domains = append(domains, dom)
	}

	return domains, nil
}

func DomainNew(db *sql.DB, domain, icon string) error {
	_, err := db.Exec(`
		INSERT INTO dns_domain(domain, icon)
		VALUES($1, $2)
		`,
		domain,
		icon,
	)

	return err
}

func DomainEdit(db *sql.DB, id int, domain, icon string) error {
	var (
		set    = cond.NewGroup()
		values = []interface{}{id}
	)

	if domain != "" {
		set.Add(",", cond.P("domain = $%d"))
		values = append(values, domain)
	}

	if icon != "" {
		set.Add(",", cond.P("icon = $%d"))
		values = append(values, icon)
	}

	if len(values) <= 0 {
		return nil
	}

	i := 2
	_, err := db.Exec(
		fmt.Sprintf(`
			UPDATE dns_domain
			SET %s
			WHERE id = $1
			`,
			set.Eval(&i),
		),
		values...,
	)

	return err
}
