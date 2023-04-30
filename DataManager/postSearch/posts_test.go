package postSearch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kycklingar/PBooru/DataManager/search"
)

func TestPostSearch(t *testing.T) {
	and := NewPtm("a1", 2)
	and2 := NewPtm("a2", 8)
	or := NewPtm("o", 10, 11, 12, 13)
	filter := NewPtm("f", 4, 5)
	unless := NewPtm("u", 3)
	p := NewPostsSearch("p")

	p.
		Join(&and).
		On(
			search.JoinCondition(
				p.Field("id"),
				and.Field("post_id"),
			),
		)
	p.Join(&and2).
		On(
			search.JoinCondition(
				p.Field("id"),
				and2.Field("post_id"),
			),
		)
	p.Join(&or).
		On(
			search.JoinCondition(
				p.Field("id"),
				filter.Field("post_id"),
			),
		)
	p.LeftJoin(&filter).
		On(
			search.JoinCondition(
				p.Field("id"),
				filter.Field("post_id"),
			),
		).
		And(&filter)
	p.LeftJoin(&unless).
		On(
			search.JoinCondition(
				filter.Field("post_id"),
				unless.Field("post_id"),
			),
		).
		And(&unless)

	g := search.Group(search.IsNull(filter.Field("post_id")))
	g.Or(search.IsNotNull(unless.Field("post_id")))

	p.Where(&and).
		And(&and2).
		And(&or).
		And(g)

	var b strings.Builder
	p.Eval(&b)
	fmt.Println(b.String())
}
