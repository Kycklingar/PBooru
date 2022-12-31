package db

import (
	"context"
	"database/sql"
)

type (
	Scanner    func(...any) error
	rowScanner func(Scanner) error
	rows       func(rowScanner) error
	RowsQuery  interface {
		Query(string, ...any) (*sql.Rows, error)
	}
	RowsQueryContext interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}
)

func QueryRows(q RowsQuery, query string, values ...any) rows {
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

func QueryRowsContext(ctx context.Context, q RowsQueryContext, query string, values ...any) rows {
	return func(scan rowScanner) error {
		rows, err := q.QueryContext(ctx, query, values...)
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
