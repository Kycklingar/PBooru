package DataManager

import (
	"testing"
)

//func TestAddParent(t *testing.T) {
//	openTestDB(t)
//
//	var tagA, tagB *Tag
//
//	tagA = NewTag()
//	tagB = NewTag()
//
//	tagA.Parse("namespace:tag a")
//	tagB.Parse("tag b")
//
//	// Test a simple add with no conflicts
//	if err := tagA.AddParent(DB, tagB); err != nil {
//		t.Fatal(err)
//	}
//
//	// Test an add with conflicting tags, ie. when the tag b already is a parent of tag a
//	if err := tagA.AddParent(DB, tagB); err == nil {
//		t.Errorf("Parent was addedd to tag a despite already being present")
//	}
//
//	// Test circular parenting, ie. a->b and b->a
//	if err := tagB.AddParent(DB, tagA); err != nil {
//		t.Error(err)
//	}
//
//	// Add child tag to a post and check if the parent is addedd to
//	p := testNewPost(t)
//
//	u := NewUser()
//	u.ID = 1
//	if err = p.EditTagsQ(DB, u, "tag b", ""); err != nil {
//		t.Fatal(err)
//	}
//
//}

func BenchmarkParseTags(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var tc = new(TagCollector)
		err := tc.ParseEscape("test, tag\\, tag, tag \\, tag \\, tag, tag tag tag, tag\\,", ',')
		//err := tc.ParseEscape("test", ',')
		if err != nil {
			b.Log(err)
		}
	}
}
