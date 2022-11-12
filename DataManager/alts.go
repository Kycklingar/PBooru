package DataManager

import (
	"database/sql"
	"fmt"
	"strings"

	mm "github.com/kycklingar/MinMax"
	"github.com/kycklingar/PBooru/DataManager/query"
	"github.com/kycklingar/set"
)

func join[T any](delim string, values []T) string {
	if len(values) < 1 {
		return ""
	}

	var b strings.Builder

	b.WriteString(fmt.Sprint(values[0]))

	for _, v := range values[1:] {
		b.WriteString(delim)
		b.WriteString(fmt.Sprint(v))
	}

	return b.String()
}

func SetAlts(posts []int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var (
			pids  = set.NewOrdered[int]()
			maxID int
		)

		err = query.Rows(
			tx,
			fmt.Sprintf(`
				SELECT id
				FROM posts
				WHERE alt_group IN(
					SELECT alt_group
					FROM posts
					WHERE id IN(
						SELECT DISTINCT COALESCE(post_id, id)
						FROM posts
						LEFT JOIN duplicates
						ON id = dup_id
						WHERE id IN(%s)
					)
				)
				GROUP BY id
				`,
				strSep(posts, ","),
			),
		)(func(scan scanner) error {
			var pid int
			err := scan(&pid)
			maxID = mm.Max(maxID, pid)
			pids.Set(pid)
			return err
		})
		if err != nil {
			return
		}

		_, err = tx.Exec(
			fmt.Sprintf(`
				UPDATE posts
				SET alt_group = $1
				WHERE id IN(%s)
				`,
				join(",", pids.Slice()),
			),
			maxID,
		)
		if err != nil {
			return
		}

		// reset count cache of the tags on all posts
		err = postsTags(tx, pids.Slice()).purgeCountCache(tx).err
		if err != nil {
			return
		}

		err = clearEmptySearchCountCache(tx)
		if err != nil {
			return
		}

		l.addTable(lPostAlts)
		l.fn = logAlts{
			pids: pids.Slice(),
		}.log

		return
	}
}

// Split the altgroup into two separate groups
// one with the supplied post ids
// the other with the remaining alts
func SplitAlts(posts []int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var (
			aMax int
			bMax int
			a    = set.NewOrdered[int]()
			b    = set.NewOrdered[int]()
		)

		b.Set(posts...)

		err = query.Rows(
			tx,
			fmt.Sprintf(`
				SELECT id
				FROM posts
				WHERE alt_group = (
					SELECT alt_group
					FROM posts
					WHERE id = $1
				)
				AND id NOT IN(%s)
				`,
				join(",", b.Slice()),
			),
			posts[0],
		)(func(scan scanner) error {
			var id int
			err := scan(&id)
			aMax = mm.Max(aMax, id)
			a.Set(id)
			return err
		})
		if err != nil {
			return
		}

		for _, p := range b.Slice() {
			bMax = mm.Max(bMax, p)
		}

		update := func(newAltGroup int, pids set.Ordered[int]) error {
			_, err := tx.Exec(
				fmt.Sprintf(`
					UPDATE posts
					SET alt_group = $1
					WHERE id IN(%s)
					`,
					join(",", pids.Slice()),
				),
				newAltGroup,
			)
			return err
		}

		if err = update(aMax, a); err != nil {
			return
		}

		if err = update(bMax, b); err != nil {
			return
		}

		// reset count cache of the tags on all posts
		err = postsTags(tx, a.Slice()).purgeCountCache(tx).err
		if err != nil {
			return
		}

		err = postsTags(tx, b.Slice()).purgeCountCache(tx).err
		if err != nil {
			return
		}

		err = clearEmptySearchCountCache(tx)
		if err != nil {
			return
		}

		l.addTable(lPostAlts)
		l.fn = logAltsSplit{
			a: logAlts{
				pids: a.Slice(),
			},
			b: logAlts{
				pids: b.Slice(),
			},
		}.log

		return
	}
}

//func (p *Post) SetAlt(q querier, altof int, user *User) error {
//	err := p.setAlt(q, altof)
//	if err != nil {
//		return err
//	}
//
//	return logAlt(q, p.ID, altof, user)
//}
//
//func (p *Post) setAlt(q querier, b int) error {
//	_, err := q.Exec(`
//		WITH d1 AS (
//			SELECT * FROM get_dupe($1)
//		),
//		d2 AS (
//			SELECT * FROM get_dupe($2)
//		),
//		altgroups AS (
//			SELECT *
//			FROM posts
//			WHERE alt_group IN(
//				SELECT alt_group
//				FROM posts
//				WHERE id IN(
//					(SELECT * FROM d1),
//					(SELECT * FROM d2)
//				)
//			)
//		)
//
//		UPDATE posts
//		SET alt_group = (
//			SELECT MAX(id)
//			FROM altgroups
//		)
//		WHERE id IN(
//			SELECT id
//			FROM altgroups
//		)
//		`,
//		p.ID,
//		b,
//	)
//
//	tc := new(TagCollector)
//	tc.GetFromPost(q, p)
//	for _, tag := range tc.Tags {
//		resetCacheTag(q, tag.ID)
//	}
//
//	c.Cache.Purge("TPC", strconv.Itoa(p.ID))
//
//	return err
//}
//
//func (p *Post) RemoveAlt(q querier, user *User) (err error) {
//	if err = p.removeAlt(q); err != nil {
//		return
//	}
//
//	return logAlt(q, p.ID, 0, user)
//}
//
//func (p *Post) removeAlt(q querier) error {
//	// Reassign alt_group id's away from the removed
//	_, err := q.Exec(`
//		UPDATE posts
//		SET alt_group = (
//			SELECT MAX(id)
//			FROM posts
//			WHERE alt_group = $1
//			AND id != $1
//		) WHERE alt_group = $1
//		`,
//		p.ID,
//	)
//	if err != nil {
//		return err
//	}
//
//	// Reset to default
//	_, err = q.Exec(`
//		UPDATE posts
//		SET alt_group = id
//		WHERE id = $1
//		`,
//		p.ID,
//	)
//
//	tc := new(TagCollector)
//	tc.GetFromPost(q, p)
//	for _, tag := range tc.Tags {
//		resetCacheTag(q, tag.ID)
//	}
//
//	c.Cache.Purge("TPC", strconv.Itoa(p.ID))
//
//	return err
//
//}
//
//func logAlt(q querier, a, b int, user *User) (err error) {
//	// Null b indicates removal
//	if b == 0 {
//		_, err = q.Exec(`
//			INSERT INTO log_alts(
//				user_id,
//				alt_a
//			)
//			VALUES(
//				$1,
//				(SELECT * FROM get_dupe($2))
//			)`,
//			user.ID,
//			a,
//		)
//		return
//	}
//	_, err = q.Exec(`
//		INSERT INTO log_alts(
//			user_id,
//			alt_a,
//			alt_b
//		)
//		VALUES (
//			$1,
//			(SELECT * FROM get_dupe($2)),
//			(SELECT * FROM get_dupe($3))
//		)`,
//		user.ID,
//		a,
//		b,
//	)
//	return
//}
