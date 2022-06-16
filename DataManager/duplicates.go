package DataManager

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	c "github.com/kycklingar/PBooru/DataManager/cache"
)

type Dupe struct {
	Post     *Post
	Inferior []*Post
}

func getDupeFromPost(q querier, p *Post) (Dupe, error) {
	var dup Dupe
	dup.Post = p
	rows, err := q.Query(`
		SELECT post_id, dup_id
		FROM duplicates
		WHERE post_id = $1
		OR post_id = (
			SELECT post_id
			FROM duplicates
			WHERE dup_id = $1
		)
		`,
		p.ID,
	)
	if err != nil {
		return dup, err
	}
	defer rows.Close()

	for rows.Next() {
		var np = NewPost()
		var bp = NewPost()

		err = rows.Scan(&bp.ID, &np.ID)
		if err != nil {
			return dup, err
		}

		if dup.Post.ID != bp.ID {
			dup.Post = bp
		}
		dup.Inferior = append(dup.Inferior, np)
	}

	return dup, nil
}

func AssignDuplicates(dupe Dupe, user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	var a func() error
	a = tx.Rollback

	defer func() {
		if err := a(); err != nil {
			log.Println(err)
		}
	}()

	ua := UserAction(user)

	// Update alternatives
	// must happen before dupe updates
	if err = updateAlts(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Handle conflicts
	if err = conflicts(tx, dupe); err != nil {
		return err
	}

	dup, err := getDupeFromPost(tx, dupe.Post)
	if err != nil {
		return err
	}

	dupe.Post = dup.Post

	if err = updateDupes(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	var values string
	for _, p := range dupe.Inferior {
		if values != "" {
			values += ","
		}
		values += fmt.Sprintf("(%d, %d)", dupe.Post.ID, p.ID)
	}

	var query = fmt.Sprint("INSERT INTO duplicates(post_id, dup_id) VALUES", values)
	_, err = tx.Exec(query)
	if err != nil {
		log.Println(err)
		return err
	}

	// Find tags in common to superior
	var cmnTags map[int]int
	cmnTags, err = commonTags(tx, dupe)
	if err != nil {
		log.Println(err)
		return err
	}

	// For each inferior post, move tags to superior
	if err = moveTags(tx, dupe, ua); err != nil {
		log.Println(err)
		return err
	}

	// Recount common tags
	for k, v := range cmnTags {
		if v >= 1 {
			var tag = &Tag{ID: k}
			tag.updateCount(tx, -v)
		}
	}

	// Replace comics with new post
	if err = replaceComicPages(tx, user, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Move votes to the new post
	if err = moveVotes(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Move views
	if err = moveViews(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Move user pool posts
	if err = movePoolPosts(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	for _, p := range dupe.Inferior {
		if err = p.Remove(tx); err != nil {
			log.Println(err)
			return err
		}
	}

	// TODO
	// Move post descriptions and comments

	// Update apple trees
	if err = updateAppleTrees(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Update reports
	if err = updateDupeReports(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	err = ua.exec(tx)
	if err != nil {
		return err
	}

	// Reset tag cache
	tc := new(TagCollector)
	tc.GetFromPost(tx, dupe.Post)
	for _, tag := range tc.Tags {
		resetCacheTag(tx, tag.ID)
	}

	a = tx.Commit

	c.Cache.Purge("TPC", strconv.Itoa(dupe.Post.ID))
	return nil
}

func updateAlts(tx querier, dupe Dupe) error {
	// TODO: fix
	//for _, p := range dupe.Inferior {
	//	if err := p.setAlt(tx, dupe.Post.ID); err != nil {
	//		return err
	//	}
	//	// Reset the inferior altgroup
	//	if err := p.removeAlt(tx); err != nil {
	//		return err
	//	}
	//}

	return nil
}

func updateAppleTrees(tx querier, dupe Dupe) error {
	var err error
	for _, p := range dupe.Inferior {

		_, err = tx.Exec(`
			UPDATE apple_tree
			SET apple = $1
			WHERE apple = $2
			AND pear NOT IN(
				SELECT pear
				FROM apple_tree
				WHERE apple = $2
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE apple_tree
			SET pear = $1
			WHERE pear = $2
			AND apple NOT IN(
				SELECT apple
				FROM apple_tree
				WHERE pear = $2
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE apple_tree
			SET processed = CURRENT_TIMESTAMP
			WHERE processed IS NULL
			AND (
				apple = $1
				OR pear = $1
			)
			`,
			p.ID,
		)
		if err != nil {
			return err
		}

	}

	return nil
}

func updateDupeReports(tx querier, dupe Dupe) (err error) {
	// Update pluck reports
	// replace fruit == dupe with post
	// A
	// B, [C], D, E -> B, [E], D, E
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE duplicate_report_posts
			SET post_id = $1
			WHERE id IN (
				SELECT drp.id
				FROM duplicate_report_posts drp
				JOIN duplicate_report dr
				ON dr.id = drp.report_id
				WHERE drp.post_id = $2
				AND dr.approved IS NULL
				AND dr.report_type = 1
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}
	}

	// Remove duplicate report fruits
	// A
	// B, [E], D, E
	_, err = tx.Exec(`
		DELETE FROM duplicate_report_posts a
		USING duplicate_report_posts b
		WHERE a.report_id = b.report_id
		AND a.post_id = b.post_id
		AND a.id < b.id
		`,
	)
	if err != nil {
		return
	}

	// Replace inferior with d.p
	// [A] -> [E]
	// B, C, D
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE duplicate_report
			SET post_id = $1
			WHERE post_id = $2
			AND approved IS NULL
			AND report_type = 1
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}
	}

	// remove pluck == fruit
	// A
	// [A], B, C
	_, err = tx.Exec(`
		DELETE FROM duplicate_report_posts
		WHERE id IN (
			SELECT drp.id
			FROM duplicate_report_posts drp
			JOIN duplicate_report dr
			ON dr.id = drp.report_id
			WHERE drp.post_id = dr.post_id
		)
		`,
	)
	if err != nil {
		return
	}

	// Remove reports without fruit
	// A
	// -
	_, err = tx.Exec(`
		DELETE FROM duplicate_report
		WHERE id IN (
			SELECT dr.id
			FROM duplicate_report dr
			LEFT JOIN duplicate_report_posts drp
			ON dr.id = drp.report_id
			WHERE drp.id IS NULL
			AND approved IS NULL
		)
		`,
	)

	return
}

func movePoolPosts(tx querier, dupe Dupe) (err error) {
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE pool_mappings
			SET post_id = $1
			WHERE post_id = $2
			AND pool_id NOT IN(
				SELECT pool_id
				FROM pool_mappings
				WHERE post_id = $1
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}

		_, err = tx.Exec(`
			DELETE FROM pool_mappings
			WHERE post_id = $1
			`,
			p.ID,
		)
		if err != nil {
			return
		}
	}

	return
}

func moveVotes(tx querier, dupe Dupe) (err error) {
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE post_score_mapping
			SET post_id = $1
			WHERE post_id = $2
			AND user_id NOT IN(
				SELECT user_id
				FROM post_score_mapping
				WHERE post_id = $1
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}

		_, err = tx.Exec(`
			DELETE FROM post_score_mapping
			WHERE post_id = $1
			`,
			p.ID,
		)
		if err != nil {
			return err
		}
	}

	return
}

func moveViews(tx querier, dupe Dupe) error {
	stmt, err := tx.Prepare(`
		UPDATE post_views
		SET post_id = $1
		WHERE post_id = $2
	`)
	defer stmt.Close()

	for _, p := range dupe.Inferior {
		_, err = stmt.Exec(dupe.Post.ID, p.ID)
		if err != nil {
			return err
		}
		if err = p.updateScore(tx); err != nil {
			return err
		}

	}

	return dupe.Post.updateScore(tx)
}

func commonTags(tx querier, dupe Dupe) (map[int]int, error) {
	var pids string
	for _, p := range dupe.Inferior {
		pids += fmt.Sprint(p.ID, ",")
	}

	pids += fmt.Sprint(dupe.Post.ID)

	rows, err := tx.Query(
		fmt.Sprintf(`
			SELECT tag_id
			FROM post_tag_mappings
			WHERE post_id IN(%s)
			`,
			pids,
		),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tids = make(map[int]int)

	for rows.Next() {
		var tid int
		if err = rows.Scan(&tid); err != nil {
			return nil, err
		}

		if _, ok := tids[tid]; !ok {
			tids[tid] = 0
		} else {
			tids[tid]++
		}
	}

	return tids, nil
}

//func moveTags(tx querier, dupe Dupe, ua *UserActions) (err error) {
//	for _, p := range dupe.Inferior {
//		_, err = tx.Exec(`
//			INSERT INTO post_tag_mappings (post_id, tag_id)
//				SELECT $1, tag_id FROM post_tag_mappings
//				WHERE post_id = $2
//			ON CONFLICT DO NOTHING
//			`,
//			dupe.Post.ID,
//			p.ID,
//		)
//		if err != nil {
//			log.Println(err)
//			return
//		}
//
//		_, err = tx.Exec(`
//			DELETE FROM post_tag_mappings
//			WHERE post_id = $1
//			`,
//			p.ID,
//		)
//		if err != nil {
//			log.Println(err)
//			return
//		}
//	}
//
//	return
//}

func moveTags(tx querier, dupe Dupe, ua *UserActions) error {
	set, err := postTags(tx, dupe.Post.ID).unwrap()
	if err != nil {
		return err
	}

	var newSet tagSet

	stmt, err := tx.Prepare(`
		DELETE FROM post_tag_mappings
		WHERE post_id = $1
		`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, inf := range dupe.Inferior {
		infset, err := postTags(tx, inf.ID).unwrap()
		if err != nil {
			return err
		}

		_, err = stmt.Exec(inf.ID)
		if err != nil {
			return err
		}

		ua.Add(nullUA(logger{
			tables: []logtable{lPostTags},
			fn: logPostTags{
				PostID:  inf.ID,
				Removed: infset,
			}.log,
		}))

		newSet = append(newSet, infset.diff(newSet)...)
	}

	newSet = newSet.diff(set)

	err = prepPTExec(tx, queryInsertPTM, dupe.Post.ID, newSet)
	if err != nil {
		return err
	}

	ua.Add(nullUA(logger{
		tables: []logtable{lPostTags},
		fn: logPostTags{
			PostID: dupe.Post.ID,
			Added:  newSet,
		}.log,
	}))

	return nil
}

// FIXME
func replaceComicPages(tx querier, user *User, dupe Dupe) (err error) {
	//exec := func(inferior *Post) ([]*ComicPost, error) {
	//	rows, err := tx.Query(`
	//		UPDATE comic_mappings
	//		SET post_id = $1
	//		WHERE post_id = $2
	//		RETURNING id, chapter_id, post_order
	//		`,
	//		dupe.Post.ID,
	//		inferior.ID,
	//	)
	//	if err != nil {
	//		return nil, err
	//	}
	//	defer rows.Close()

	//	var cps []*ComicPost

	//	for rows.Next() {
	//		var cp = newComicPost()
	//		cp.Post = dupe.Post

	//		err = rows.Scan(&cp.ID, &cp.Chapter.ID, &cp.Order)
	//		if err != nil {
	//			return nil, err
	//		}

	//		cps = append(cps, cp)
	//	}

	//	return cps, nil
	//}

	//for _, p := range dupe.Inferior {
	//	var cps []*ComicPost
	//	cps, err = exec(p)
	//	if err != nil {
	//		return err
	//	}

	//	// Log changes
	//	for _, cp := range cps {
	//		err = cp.log(tx, lUpdate, user)
	//		if err != nil {
	//			return
	//		}
	//	}
	//}

	return
}

func updateDupes(tx querier, dupe Dupe) error {
	for _, p := range dupe.Inferior {
		dup, err := getDupeFromPost(tx, p)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE duplicates
			SET post_id = $1
			WHERE post_id = $2
			`,
			dupe.Post.ID,
			dup.Post.ID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func conflicts(tx querier, dupe Dupe) error {
	in := func(dups []Dupe, p *Post) bool {
		for _, d := range dups {
			if d.Post.ID == p.ID {
				return true
			}
		}

		return false
	}

	var newSets []Dupe
	for _, p := range dupe.Inferior {
		dup, err := getDupeFromPost(tx, p)
		if err != nil {
			return err
		}

		if !in(newSets, dup.Post) {
			newSets = append(newSets, dup)
		}
	}

	superiors := func() bool {
		for _, p := range dupe.Inferior {
			for _, dupSet := range newSets {
				if dupSet.Post.ID == p.ID {
					return true
				}
			}
		}

		return false
	}

	bdupe, err := getDupeFromPost(tx, dupe.Post)
	if err != nil {
		return err
	}

	// There are two sets
	// A > B
	// D > E

	// Do these new sets work
	// B > C
	// Yes. Replace B with A > C
	//if bdupe.Post.ID != dupe.Post.ID && superiors() {
	//	return nil
	//}

	//// B > D
	//// Yes. Replace B with A > D
	//if bdupe.Post.ID != dupe.Post.ID && superiors() {
	//	return nil
	//}

	//// C > B
	//// No. Replace B with A, we don't know if C > A
	//if len(bdupe.Inferior) <= 0 && !superiors() {
	//	return error
	//}

	//// B > E
	//// No. Replace B with A and E with D, we don't know if A > D
	//if bdupe.Post.ID != dupe.Post.ID && !superiors() {
	//	return error
	//}

	// Will alway result in conflict
	if !superiors() {
		var derr DupeConflict

		derr.NeedChecking = append(derr.NeedChecking, bdupe.Post)

		for _, set := range newSets {
			derr.NeedChecking = append(derr.NeedChecking, set.Post)
		}

		return derr
	}

	// Don't want to assign a post to itself
	for _, set := range newSets {
		if set.Post.ID == bdupe.Post.ID {
			return errors.New("Conflicting dupe assignment. Trying to assign post as a dupe of itself")
		}
	}

	return nil
}

type DupeConflict struct {
	NeedChecking []*Post
}

func (d DupeConflict) Error() string {
	var str string
	for _, p := range d.NeedChecking {
		str += fmt.Sprint(p.ID, " ")
	}
	return fmt.Sprint("Duplicate conflict. Please compare: ", str)
}
