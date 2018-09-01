package DataManager

import "testing"

func TestTagSave(t *testing.T) {
	db := openTestDB(t)

	tag := NewTag()

	err := tag.Parse("test:tag")
	if err != nil {
		t.Error(err)
		return
	}

	err = tag.Save(db)
	if err != nil {
		t.Error(err)
		return
	}

	tag = NewTag()

	tag.Parse("test:tag")

	if tag.ID(db) == 0 {
		t.Errorf("Tag Save failed: Invalid tag, ID=%d, TAG=%s, NAMESPACE=%s", tag.ID(db), tag.Tag(db), tag.Namespace(db).Namespace(db))
	}
}
