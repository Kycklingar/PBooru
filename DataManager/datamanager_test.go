package DataManager

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

const (
	dbDriver         = "postgres"
	connectionString = "user=pbdb password=1234 dbname=pbdbtest sslmode=disable"
)

func openTestDB(t *testing.T) *sql.DB {
	if DB != nil {
		return DB
	}

	var err error

	if DB, err = sql.Open(dbDriver, connectionString); err != nil {
		t.Fatal("Failed to open testdb", err)
	}

	if err = update(DB, "../out/sql"); err != nil {
		t.Fatal("Failed to update testdb", err)
	}
	return DB
}
