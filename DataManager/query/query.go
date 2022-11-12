package query

import "database/sql"

type (
	Scanner    func(...any) error
	rowScanner func(Scanner) error
	rows       func(rowScanner) error
	RowsQuery  interface {
		Query(string, ...any) (*sql.Rows, error)
	}
)

func Rows(q RowsQuery, query string, values ...any) rows {
	return func(scan rowScanner) error {
		rows, err := q.Query(query, values...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			err = scan(rows.Scan)
			if err != nil {
				return err
			}
		}

		return rows.Err()
	}
}
