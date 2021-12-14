package DataManager

import (
	"database/sql"
	"fmt"
)

func SetAlts(posts []int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		rows, err := tx.Query(
			fmt.Sprintf(`
				SELECT id, alt_group
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
		)
		if err != nil {
			return
		}
		defer rows.Close()

		var (
			pids  []int
			alts  []int
			maxID int
		)

		for rows.Next() {
			var pid, alt int

			err = rows.Scan(&pid, &alt)
			if err != nil {
				return
			}

			if pid > maxID {
				maxID = pid
			}

			pids = append(pids, pid)
			alts = append(alts, alt)
		}

		_, err = tx.Exec(
			fmt.Sprintf(`
				UPDATE posts
				SET alt_group = $1
				WHERE id IN(%s)
				`,
				strSep(pids, ","),
			),
			maxID,
		)
		if err != nil {
			return
		}

		l.table = lPostAlts
		l.fn = logAlts{
			AltGroup: maxID,
			pids:     pids,
		}.log

		return
	}
}

// Split the altgroup into two separate groups
// one with the supplied post ids
// the other with the remaining alts
func SplitAlts(posts []int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		rows, err := tx.Query(
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
				strSep(posts, ","),
			),
			posts[0],
		)
		if err != nil {
			return
		}
		defer rows.Close()

		var (
			aMax int
			bMax int
			a    []int
		)

		for rows.Next() {
			var p int
			err = rows.Scan(&p)
			if err != nil {
				return
			}

			if aMax < p {
				aMax = p
			}

			a = append(a, p)
		}

		for _, p := range posts {
			if p > bMax {
				bMax = p
			}
		}

		update := func(newAltGroup int, pids []int) error {
			_, err := tx.Exec(
				fmt.Sprintf(`
					UPDATE posts
					SET alt_group = $1
					WHERE id IN(%s)
					`,
					strSep(pids, ","),
				),
				newAltGroup,
			)
			return err
		}

		if err = update(aMax, a); err != nil {
			return
		}

		if err = update(bMax, posts); err != nil {
			return
		}

		l.table = lPostAlts
		l.fn = logAltsSplit{
			a: logAlts{
				AltGroup: aMax,
				pids:     a,
			},
			b: logAlts{
				AltGroup: bMax,
				pids:     posts,
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
