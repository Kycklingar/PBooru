package db

import (
	"database/sql"
)

// Standard set of database operations
type Q interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Prepare(string) (*sql.Stmt, error)
}
