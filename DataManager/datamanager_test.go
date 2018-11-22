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

var testDB *sql.DB

func openTestDB(t *testing.T) *sql.DB {
	if testDB != nil {
		return testDB
	}

	var err error

	if testDB, err = sql.Open(dbDriver, connectionString); err != nil {
		t.Error("Failed to open testdb", err)
		return nil
	}

	if err = update(testDB, "../out/sql"); err != nil {
		t.Error("Failed to update testdb", err)
		return nil
	}
	return testDB
}
