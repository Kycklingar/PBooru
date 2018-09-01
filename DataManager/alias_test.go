package DataManager

import (
	"database/sql"
	"log"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

var testDB *sql.DB

func openTestDB(t *testing.T) *sql.DB {
	if testDB != nil {
		return testDB
	}
	var err error
	testDB, err = sql.Open("sqlite3", "../testDB.db")
	if err != nil {
		t.Error("Failed to open testDB.db")
	}
	err = update(testDB, "../sql")
	if err != nil {
		t.Error("Failed to update testDB.db")
		return nil
	}
	return testDB
}

func TestAliasSave(t *testing.T) {
	log.SetFlags(log.Llongfile)
	db := openTestDB(t)

	al := NewAlias()

	al.Tag().Parse("new")
	al.To(db).Parse("al")

	err := al.Save(db)
	if err != nil {
		t.Error("Saving alias failed: ", err)
	}

	al = NewAlias()

	al.Tag().Parse("new")

	if al.Tag().ID(db) == 0 {
		t.Error("Save alias failed: tag doesn't exist")
	}

	if al.To(db).ID(db) == 0 {
		t.Error("Save alias failed: to doesn't exist")
	}

	al = NewAlias()
	al.Tag().Parse("al")

	if al.From(db) == nil {
		t.Error("Save alias failed: from is empty")
	}

}

func TestAliasFrom(t *testing.T) {
	db := openTestDB(t)

	al := NewAlias()
	al.Tag().Parse("al")

	if al.From(db) == nil {
		t.Error("Alias from is nil, expecting 1 result")
	}

	for _, tag := range al.From(db) {
		if tag.ID(db) == 0 {
			t.Error("Alias from tag is empty")
		}
	}
}

func TestAliasTo(t *testing.T) {
	db := openTestDB(t)

	al := NewAlias()
	al.Tag().Parse("new")

	if al.To(db).ID(db) == 0 {
		t.Error("Alias to is empty")
	}
}
