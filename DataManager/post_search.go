package DataManager

import (
	"fmt"

	"github.com/kycklingar/set"
	"github.com/kycklingar/sqhell/cond"
)

type group struct {
	//nested []group

	and       set.Sorted[Tag]
	or        set.Sorted[Tag]
	filter    set.Sorted[Tag]
	unless    set.Sorted[Tag]
	tombstone bool
}

func searchGroup(and, or, filter, unless set.Sorted[Tag], tombstone bool) group {
	return group{
		and,
		or,
		filter,
		unless,
		tombstone,
	}
}

func (g group) sel(where *cond.Group) string {
	//var q, where, trail string

	var (
		joins = cond.NewGroup().Add("", cond.N("posts p"))
	)

	//where = fmt.Sprintf("WHERE %s\n", wh)

	//if wh != "" {
	//	trail = "AND"
	//}

	if !g.tombstone && !(len(g.or) > 0 || len(g.and) > 0 || len(g.filter) > 0) {
		return joins.Eval(nil) + "\nWHERE " + where.Eval(nil)
	}

	if len(g.or) > 0 {
		joins.Add(
			"\nJOIN",
			cond.N(
				fmt.Sprintf(`
					(
						SELECT DISTINCT post_id
						FROM post_tag_mappings
						WHERE tag_id IN(%s)
					) o
					ON p.id = o.post_id
					`,
					tSetStr(g.or),
				),
			),
		)

		//where += fmt.Sprintf("%s o.tag_id IN(%s)\n", trail, sep(g.or, ","))

		//trail = "AND"
	}

	if len(g.and) > 0 {
		var join, w string
		for i := 1; i < len(g.and); i++ {
			join += fmt.Sprintf(`
				JOIN post_tag_mappings a%d
				ON p.id = a%d.post_id
				`,
				i+1,
				i+1,
			)
			w += fmt.Sprintf("AND a%d.tag_id = %d\n", i+1, g.and[i].ID)
		}

		joins.Add(
			"\nJOIN",
			cond.N(
				fmt.Sprintf(`
					post_tag_mappings a1
					ON p.id = a1.post_id
					%s
					`,
					join,
				),
			),
		)

		where.Add(
			"\nAND",
			cond.N(
				fmt.Sprintf(`
				a1.tag_id = %d
				%s
				`,
					g.and[0].ID,
					w,
				),
			))

		//trail = "AND"
	}

	if len(g.filter) > 0 {

		var unlessJ, unlessW string

		if len(g.unless) > 0 {
			unlessJ = fmt.Sprintf(`
				LEFT JOIN post_tag_mappings u
				ON f.post_id = u.post_id
				AND u.tag_id IN(%s)
				`,
				tSetStr(g.unless),
			)

			unlessW = "AND u.post_id IS NULL"
		}

		joins.Add(
			"\nLEFT JOIN",
			cond.N(
				fmt.Sprintf(`
				(
					SELECT f.post_id
					FROM post_tag_mappings f
					%s
					WHERE
					f.tag_id IN(%s)
					%s
				) f
				ON p.id = f.post_id
				`,
					unlessJ,
					tSetStr(g.filter),
					unlessW,
				),
			),
		)

		where.Add("\nAND", cond.N("f.post_id IS NULL"))
	}

	if g.tombstone {
		joins.Add(
			"\nLEFT JOIN",
			cond.N("tombstone ts ON p.id = ts.post_id"),
		)

		where.Add("\nAND", cond.N("ts.post_id IS NOT NULL"))
	}

	return joins.Eval(nil) + "\nWHERE " + where.Eval(nil)
}

/* Failed grouping attempt
func (g group) sel() string {
	if !(
		len(g.or) > 0
		|| len(g.and) > 0
		|| len(g.filter) > 0
	){
		return ""
	}

	q := fmt.Sprintf(`
		SELECT DISTINCT post_id
		FROM post_tag_mappings
		WHERE
		`
	)

	// Trailing operator
	var firstie string

	if len(g.or) > 0 {
		var ors string
		for _, v := range g.or {
			fmt.Sprint(ors, v, ",")
		}
		q += fmt.Sprintf(`
			post_id IN(
				SELECT post_id
				FROM post_tag_mappings
				WHERE tag_id IN(%s)
			)
			`,
			ors
		)

		firstie = "OR"
	}

	if len(g.and) > 0 {
		var join, where string
		for i := 1; i < len(g.and); i++){
			join += fmt.Sprintf(`
				JOIN post_tag_mappings p%d
				ON p1.post_id = p%d.post_id
				`,
				i,
				i,
			)
			where += fmt.Sprintf("AND %d.tag_id = %d\n", g.and[i])
		}
		q += fmt.Sprintf(`
			%s post_id IN(
				SELECT post_id
				FROM post_tag_mappings p1
				%s
				WHERE p1.tag_id = %d
				%s
			)
			`,
			firstie,
			join,
			g.and[0],
			where,
		)

		firstie = "AND"
	}

	for _, v := range g.nested {
		q += fmt.Sprintf(`
			%s post_id IN(
				%s
			)
			`,
			firstie,
			v.sel(),
		)

	}

	return q
}
*/

// A search like:
// (renamon, s-nina), judy hopps, comic, (krystal, sif)
// Translated:
// (renamon & s-nina) | (judy hopps | comic) | (krystal & sif)

// (renamon | judy hopps & s-nina)
// Local Group:
// (renamon | judy hopps) & s-nina

// (!bunny & (impmon | ass) | (s-nina & renamon)
