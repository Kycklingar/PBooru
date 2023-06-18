package table

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kycklingar/PBooru/DataManager/search/condition"
)

func TestTable(t *testing.T) {
	var posts = New("posts", "p")
	var ptm = New("post_tag_mappings", "ptm1")

	posts.
		LeftJoin(ptm).
		On(
			condition.Join(
				posts.Field("id"),
				ptm.Field("post_id"),
			),
		)

	posts.Where(condition.List(ptm.Field("tag_id"), 1, 2, 3))

	var b strings.Builder

	posts.Eval(&b)

	fmt.Println(b.String())
}
