package DataManager

import "testing"

func TestPostSave(t *testing.T) {
	db := openTestDB(t)

	p := NewPost()
	p.SetHash("Test")
	p.SetThumb("Thumb")
	p.Mime(db).Parse("new/mime")

	err := p.Save(db, NewUser())
	if err != nil {
		t.Error(err)
		return
	}

	p = NewPost()
	p.SetHash("Test")
	if p.ID(db) == 0 {
		t.Error("Post Save failed. post not available")
		return
	}
	if p.Thumb(db) != "Thumb" {
		t.Error("Post Save failed. post thumb incorrect")
		return
	}
}

// func TestPostSearch(t *testing.T) {
// 	db := openTestDB(t)

// 	p := PostCollector{}
// 	if err := p.Get("", false, 10, 0); err != nil {
// 		t.Error("p.Get failed", err)
// 	}

// 	if len(p.GetW(10, 0)) <= 0 {
// 		t.Error("p.Get: got:", p.GetW(10, 0))
// 	}
// }
