package DataManager

import (
	"testing"
)

func TestAliasSave(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Error("Failed to open testdb")
	}
	defer db.Close()

	a := NewAlias()
	a.Tag.Tag = "tag"
	a.Tag.Namespace.Namespace = "namespace"

	a.To.Tag = "tag2"
	a.To.Namespace.Namespace = "none"

	if err := a.Save(db); err != nil {
		t.Error("Failed to save alias", err)
	}
}
