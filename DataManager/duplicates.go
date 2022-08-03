package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Dupe struct {
	Post     *Post
	Inferior []*Post
}

func (d Dupe) strindex(i int) string {
	return strconv.Itoa(d.Inferior[i].ID)
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

type dupeProcedures func(*sql.Tx, *Dupe) error

func dprocUAWrap(f func(*sql.Tx, *Dupe, *UserActions) error, ua *UserActions) dupeProcedures {
	return func(tx *sql.Tx, dupe *Dupe) error {
		return f(tx, dupe, ua)
	}
}

func AssignDuplicates(dupe Dupe, user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	ua := UserAction(user)

	var procedures = []dupeProcedures{
		upgradeDupe,
		conflicts,
		updateDupes,
		insertDupes,
		dprocUAWrap(updateAlts, ua),
		dprocUAWrap(moveTags, ua),
		dprocUAWrap(moveMetadata, ua),
		dprocUAWrap(moveDescription, ua),
		dprocUAWrap(replaceComicPages, ua),
		moveVotes,
		moveViews,
		movePoolPosts,
		removeInferiors,
		updateAppleTrees,
		updateDupeReports,
	}

	for _, proc := range procedures {
		err = proc(tx, &dupe)
		if err != nil {
			return err
		}
	}

	err = ua.Exec()

	return err
}

func insertDupes(tx *sql.Tx, dupe *Dupe) error {
	var query = fmt.Sprintf(`
		INSERT INTO duplicates(post_id, dup_id)
		SELECT $1, UNNEST(ARRAY[%s])
		`,
		sep(",", len(dupe.Inferior), dupe.strindex),
	)
	_, err := tx.Exec(query, dupe.Post.ID)
	return err
}

// Upgrade dupe.Post to its superior
func upgradeDupe(tx *sql.Tx, dupe *Dupe) error {
	u, err := getDupeFromPost(tx, dupe.Post)
	dupe.Post = u.Post
	return err
}

func removeInferiors(tx *sql.Tx, dupe *Dupe) error {
	for _, inf := range dupe.Inferior {
		if err := inf.Remove(tx); err != nil {
			return err
		}
	}

	return nil
}

func updateAlts(tx *sql.Tx, dupe *Dupe, ua *UserActions) error {
	// Get the alt groups of the inferior
	// and merge with post alt group

	infStr := sep(", ", len(dupe.Inferior), dupe.strindex)

	// Alts of inferior to be applied to superior
	rows, err := tx.Query(
		fmt.Sprintf(`
			SELECT id
			FROM posts
			WHERE alt_group IN(
				SELECT alt_group
				FROM posts
				WHERE id IN(%s)
			)
			AND id NOT IN(%s)
			`,
			infStr,
			infStr,
		),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var alts = logAlts{
		pids: []int{dupe.Post.ID},
	}

	for rows.Next() {
		var pid int
		err = rows.Scan(&pid)
		if err != nil {
			return err
		}

		alts.pids = append(alts.pids, pid)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	if len(alts.pids) > 1 {
		ua.addLogger(logger{
			tables: []logtable{lPostAlts},
			fn:     alts.log,
		})
	}

	_, err = tx.Exec(
		fmt.Sprintf(`
		UPDATE posts
		SET alt_group = (
			SELECT alt_group
			FROM posts
			WHERE id = $1
		)
		WHERE alt_group IN(
			SELECT alt_group
			FROM posts
			WHERE id IN(%s)
		) AND id NOT IN(%s)
		`,
			infStr,
			infStr,
		),
		dupe.Post.ID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		fmt.Sprintf(`
			UPDATE posts
			SET alt_group = id
			WHERE id IN(%s)
			`,
			infStr,
		),
	)

	return err
}

func updateAppleTrees(tx *sql.Tx, dupe *Dupe) error {
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

func updateDupeReports(tx *sql.Tx, dupe *Dupe) (err error) {
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

func movePoolPosts(tx *sql.Tx, dupe *Dupe) (err error) {
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

func moveVotes(tx *sql.Tx, dupe *Dupe) (err error) {
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

func moveViews(tx *sql.Tx, dupe *Dupe) error {
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

// get tags and counts having at least two posts in common
func commonTags(tx querier, dupe *Dupe) (map[int]int, error) {
	pids := fmt.Sprint(sep(",", len(dupe.Inferior), dupe.strindex), ",", dupe.Post.ID)

	rows, err := tx.Query(
		fmt.Sprintf(`
			SELECT tag_id, count(*) - 1
			FROM post_tag_mappings
			WHERE post_id IN(%s)
			GROUP BY tag_id
			HAVING count(*) > 1
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
		var tid, count int
		if err = rows.Scan(&tid, &count); err != nil {
			return nil, err
		}

		tids[tid] = count
	}

	return tids, nil
}

func moveTags(tx *sql.Tx, dupe *Dupe, ua *UserActions) error {
	// Find tags in common to superior
	var cmnTags map[int]int
	cmnTags, err := commonTags(tx, dupe)
	if err != nil {
		return err
	}

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

		ua.addLogger(logger{
			tables: []logtable{lPostTags},
			fn: logPostTags{
				PostID:  inf.ID,
				Removed: infset,
			}.log,
		})

		newSet = append(newSet, infset.diff(newSet)...)
	}

	newSet = newSet.diff(set)

	err = prepPTExec(tx, queryInsertPTM, dupe.Post.ID, newSet)
	if err != nil {
		return err
	}

	ua.addLogger(logger{
		tables: []logtable{lPostTags},
		fn: logPostTags{
			PostID: dupe.Post.ID,
			Added:  newSet,
		}.log,
	})

	// Recount common tags
	for k, v := range cmnTags {
		var tag = &Tag{ID: k}
		tag.updateCount(tx, -v)
	}

	return nil
}

func moveMetadata(tx *sql.Tx, dupe *Dupe, ua *UserActions) error {
	var metaMap = make(metaDataMap)

	for _, inf := range dupe.Inferior {
		err := inf.QMul(tx, PFMetaData)
		if err != nil {
			return err
		}

		metaMap.merge(inf.MetaData)

		// Remove metadata from inferior
		for namespace, datas := range inf.MetaData {
			for _, data := range datas {
				switch namespace {
				case "date":
					ua.Add(postRemoveCreationDate(inf.ID, data))
				default:
					ua.Add(postRemoveMetaData(inf.ID, data))
				}
			}
		}
	}

	// Add metadata to superior
	for namespace, datas := range metaMap {
		for _, data := range datas {
			switch namespace {
			case "date":
				ua.Add(postAddCreationDate(dupe.Post.ID, data))
			default:
				ua.Add(postAddMetaData(dupe.Post.ID, data))
			}
		}
	}

	return nil
}

func moveDescription(tx *sql.Tx, dupe *Dupe, ua *UserActions) error {
	err := dupe.Post.QMul(tx, PFDescription)
	if err != nil {
		return err
	}

	var descrs = make(map[int]string)

	for _, inf := range dupe.Inferior {
		err = inf.QMul(tx, PFDescription)
		if err != nil {
			return err
		}
		if len(inf.Description) > 0 {
			descrs[inf.ID] = inf.Description
		}
	}

	if len(descrs) <= 0 {
		return nil
	}

	// Empty dest, only one source. Replace
	if len(descrs) == 1 && dupe.Post.Description == "" {
		for inf, descr := range descrs {
			ua.Add(PostChangeDescription(dupe.Post.ID, descr))
			ua.Add(PostChangeDescription(inf, ""))
		}
	} else {
		var b strings.Builder
		b.WriteString(dupe.Post.Description)

		for inf, descr := range descrs {
			fmt.Fprintf(&b, "\n\n---- Addendum of duplicate %d ----\n", inf)
			b.WriteString(descr)
			ua.Add(PostChangeDescription(inf, ""))
		}

		ua.Add(PostChangeDescription(dupe.Post.ID, b.String()))
	}

	return nil
}

func replaceComicPages(tx *sql.Tx, dupe *Dupe, ua *UserActions) (err error) {
	rows, err := tx.Query(
		fmt.Sprintf(`
			UPDATE comic_page
			SET post_id = $1
			WHERE post_id IN(%s)
			RETURNING id, chapter_id, page
			`,
			sep(", ", len(dupe.Inferior), dupe.strindex),
		),
		dupe.Post.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var lcp = logComicPage{
			Action: aModify,
			postID: dupe.Post.ID,
		}

		err = rows.Scan(
			&lcp.ID,
			&lcp.ChapterID,
			&lcp.Page,
		)
		if err != nil {
			return err
		}

		ua.addLogger(logger{
			tables: []logtable{lComicPage},
			fn:     lcp.log,
		})
	}

	return
}

// Update existing duplicates, point to new superiors
func updateDupes(tx *sql.Tx, dupe *Dupe) error {
	for _, inf := range dupe.Inferior {
		_, err := tx.Exec(`
			UPDATE duplicates
			SET post_id = $1
			WHERE post_id = $2
			`,
			dupe.Post.ID,
			inf.ID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// Ensures non of the inferiors are inferior of other dupe sets
func conflicts(tx *sql.Tx, dupe *Dupe) error {
	// there is a conflict if an inferior already is an inferior of another dupe
	// the superior of the other dupe needs to be check against the superior of
	// this dupe
	rows, err := tx.Query(
		fmt.Sprintf(`
			SELECT post_id
			FROM duplicates
			WHERE dup_id IN(%s)
			`,
			sep(",", len(dupe.Inferior), dupe.strindex),
		),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var conflict DupeConflict

	for rows.Next() {
		var sup int
		err = rows.Scan(&sup)
		if err != nil {
			return err
		}
		conflict.check = append(conflict.check, sup)
	}

	if len(conflict.check) > 0 {
		conflict.check = append([]int{dupe.Post.ID}, conflict.check...)
		return conflict
	}

	// Remove self assignments
	var inferior []*Post
	for _, inf := range dupe.Inferior {
		if inf.ID != dupe.Post.ID {
			inferior = append(inferior, inf)
		}
	}
	dupe.Inferior = inferior

	if len(dupe.Inferior) == 0 {
		return errors.New("Dupe assignment has been reduced to no duplicates")
	}

	return nil
}

type DupeConflict struct {
	check []int
}

func (d DupeConflict) Error() string {
	var str string
	for _, id := range d.check {
		str += fmt.Sprint(id, " ")
	}
	return fmt.Sprint("Duplicate conflict. Please compare: ", str)
}
