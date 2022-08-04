package DataManager

import (
	"database/sql"
	"log"
)

type (
	scanner    func(...any) error
	rowScanner func(scanner) error
	rows       func(rowScanner) error
	rowsQuery  interface {
		Query(string, ...any) (*sql.Rows, error)
	}
)

func commitOrDie(tx *sql.Tx, err *error) {
	var terr error
	if *err != nil {
		terr = tx.Rollback()
	} else {
		terr = tx.Commit()
	}

	if terr != nil {
		log.Println(err)
	}
}

func query(q rowsQuery, query string, values ...any) rows {
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
