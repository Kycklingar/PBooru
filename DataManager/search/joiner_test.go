package search

import (
	"fmt"
	"strings"
	"testing"
)

func TestJoiner(t *testing.T) {
	and := NewPtm("a1", 2)
	and2 := NewPtm("a2", 8)
	or := NewPtm("o", 10, 11, 12, 13)
	filter := NewPtm("f", 4, 5)
	unless := NewPtm("u", 3)
	p := NewPosts("p")

	p.
		Join(&and).
		On(
			JoinCondition{
				p.Field("id"),
				and.Field("post_id"),
			},
		).
		Join(&and2).
		On(
			JoinCondition{
				p.Field("id"),
				and2.Field("post_id"),
			},
		).
		Join(&or).
		On(
			JoinCondition{
				p.Field("id"),
				filter.Field("post_id"),
			},
		).
		LeftJoin(&filter).
		On(
			JoinCondition{
				p.Field("id"),
				filter.Field("post_id"),
			},
		).
		And(&filter).
		LeftJoin(&unless).
		On(
			JoinCondition{
				filter.Field("post_id"),
				unless.Field("post_id"),
			},
		).
		And(&unless)

	g := Group(IsNull(filter.Field("post_id")))
	g.Or(IsNotNull(unless.Field("post_id")))

	p.Where(&and).
		And(&and2).
		And(&or).
		And(g)

	var b strings.Builder
	p.Eval(&b)
	fmt.Println(b.String())
}
