package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	mm "github.com/kycklingar/MinMax"
	"github.com/kycklingar/PBooru/DataManager/query"
	"github.com/kycklingar/set"
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
	return dup, query.Rows(
		q,
		`SELECT post_id, dup_id
		FROM duplicates
		WHERE post_id = $1
		OR post_id = (
			SELECT post_id
			FROM duplicates
			WHERE dup_id = $1
		)
		`,
		p.ID,
	)(func(scan scanner) error {
		var (
			sup = NewPost()
			inf = NewPost()
		)

		err := scan(&sup.ID, &inf.ID)
		if err != nil {
			return err
		}

		if dup.Post.ID != sup.ID {
			dup.Post = sup
		}
		dup.Inferior = append(dup.Inferior, inf)

		return nil
	})
}

type dupeProcedure func(*sql.Tx) error

func (dupe Dupe) dprocUAWrap(f func(*sql.Tx, *UserActions) error, ua *UserActions) dupeProcedure {
	return func(tx *sql.Tx) error {
		return f(tx, ua)
	}
}

func AssignDuplicates(dupe Dupe, user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	ua := UserAction(user)

	var procedures = []dupeProcedure{
		dupe.upgradeDupe,
		dupe.conflicts,
		dupe.dprocUAWrap(dupe.updateAndInsert, ua),
		dupe.dprocUAWrap(dupe.updateAlts, ua),
		dupe.dprocUAWrap(dupe.moveTags, ua),
		dupe.dprocUAWrap(dupe.moveMetadata, ua),
		dupe.dprocUAWrap(dupe.moveDescription, ua),
		dupe.dprocUAWrap(dupe.replaceComicPages, ua),
		dupe.moveVotes,
		dupe.moveViews,
		dupe.movePoolPosts,
		dupe.removeInferiors,
		dupe.updateAppleTrees,
		dupe.updateDupeReports,
	}

	for _, proc := range procedures {
		err = proc(tx)
		if err != nil {
			return err
		}
	}

	err = ua.exec(tx)

	return err
}

func (dupe *Dupe) updateAndInsert(tx *sql.Tx, ua *UserActions) error {
	var dedups []Post

	if err := query.Rows(
		tx,
		fmt.Sprintf(`
			UPDATE duplicates
			SET post_id = $1
			WHERE post_id IN(%s)
			RETURNING dup_id
			`,
			sep(",", len(dupe.Inferior), dupe.strindex),
		),
		dupe.Post.ID,
	)(func(scan scanner) (err error) {
		var p Post
		err = scan(&p.ID)
		dedups = append(dedups, p)
		return
	}); err != nil {
		return err
	}

	_, err := tx.Exec(
		fmt.Sprintf(`
			INSERT INTO duplicates(post_id, dup_id)
			SELECT $1, UNNEST(ARRAY[%s])
			`,
			sep(",", len(dupe.Inferior), dupe.strindex),
		),
		dupe.Post.ID,
	)
	if err != nil {
		return err
	}

	for _, inf := range dupe.Inferior {
		dedups = append(dedups, *inf)
	}

	ua.addLogger(logger{
		tables: []logtable{lDuplicates},
		fn: logDuplicates{
			Action:   aCreate,
			superior: *dupe.Post,
			Inferior: dedups,
		}.log,
	})

	return nil
}

// Upgrade dupe.Post to its superior
func (dupe *Dupe) upgradeDupe(tx *sql.Tx) error {
	u, err := getDupeFromPost(tx, dupe.Post)
	dupe.Post = u.Post
	return err
}

func (dupe *Dupe) removeInferiors(tx *sql.Tx) error {
	for _, inf := range dupe.Inferior {
		if err := inf.Remove(tx); err != nil {
			return err
		}
	}

	return nil
}

func (dupe *Dupe) updateAlts(tx *sql.Tx, ua *UserActions) error {
	// Get the alt groups of the inferior
	// and merge with post alt group

	infStr := sep(", ", len(dupe.Inferior), dupe.strindex)

	var (
		max  = dupe.Post.ID
		pids = set.NewOrdered[int]()
	)

	// Alts of inferior to be applied to superior
	err := query.Rows(
		tx,
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
	)(func(scan scanner) error {
		var pid int
		err := scan(&pid)
		pids.Set(pid)
		max = mm.Max(max, pid)
		return err
	})
	if err != nil {
		return err
	}

	switch length := len(pids.Slice()); {
	case length > 1:
		ua.addLogger(logger{
			tables: []logtable{lPostAlts},
			fn: logAlts{
				pids: pids.Slice(),
			}.log,
		})
		fallthrough
	case length > 0:
		_, err = tx.Exec(
			fmt.Sprintf(`
			UPDATE posts
			SET alt_group = $1
			WHERE id IN (%s)
			`,
				join(",", pids.Slice()),
			),
			max,
		)
		if err != nil {
			return err
		}

	}

	// Reset the altgroup of inferior
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

func (dupe *Dupe) updateAppleTrees(tx *sql.Tx) error {
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

func (dupe *Dupe) updateDupeReports(tx *sql.Tx) (err error) {
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

func (dupe *Dupe) movePoolPosts(tx *sql.Tx) (err error) {
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

func (dupe *Dupe) moveVotes(tx *sql.Tx) (err error) {
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

func (dupe *Dupe) moveViews(tx *sql.Tx) error {
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
func (dupe Dupe) commonTags(tx querier) (map[int]int, error) {
	pids := fmt.Sprint(sep(",", len(dupe.Inferior), dupe.strindex), ",", dupe.Post.ID)

	var tids = make(map[int]int)
	err := query.Rows(
		tx,
		fmt.Sprintf(`
			SELECT tag_id, count(*) - 1
			FROM post_tag_mappings
			WHERE post_id IN(%s)
			GROUP BY tag_id
			HAVING count(*) > 1
			`,
			pids,
		),
	)(func(scan scanner) error {
		var tid, count int
		err := scan(&tid, &count)
		tids[tid] = count
		return err
	})
	if err != nil {
		return nil, err
	}

	return tids, nil
}

func (dupe *Dupe) moveTags(tx *sql.Tx, ua *UserActions) error {
	// Find tags in common to superior
	var cmnTags map[int]int
	cmnTags, err := dupe.commonTags(tx)
	if err != nil {
		return err
	}

	postSet, err := postTags(tx, dupe.Post.ID).unwrap()
	if err != nil {
		return err
	}

	var newSet = set.New[Tag]()

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

		newSet = set.Union(newSet, infset)
	}

	newSet = set.Diff(newSet, postSet)

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

	var changed set.Sorted[Tag]

	// Recount common tags
	for k, v := range cmnTags {
		var tag = Tag{ID: k}
		changed.Set(tag)
		tag.updateCount(tx, -v)
	}

	err = tagChain(changed).purgeCountCache(tx).err
	if err != nil {
		return err
	}

	return clearEmptySearchCountCache(tx)
}

func (dupe *Dupe) moveMetadata(tx *sql.Tx, ua *UserActions) error {
	var metaMap = make(metaDataMap)

	for _, inf := range dupe.Inferior {
		err := inf.qMetaData(tx, PFMetaData)
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

func (dupe *Dupe) moveDescription(tx *sql.Tx, ua *UserActions) error {
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

func (dupe *Dupe) replaceComicPages(tx *sql.Tx, ua *UserActions) error {
	return query.Rows(
		tx,
		fmt.Sprintf(`
			UPDATE comic_page
			SET post_id = $1
			WHERE post_id IN(%s)
			RETURNING id, chapter_id, page
			`,
			sep(", ", len(dupe.Inferior), dupe.strindex),
		),
		dupe.Post.ID,
	)(func(scan scanner) error {
		var lcp = logComicPage{
			Action: aModify,
			postID: dupe.Post.ID,
		}

		err := scan(
			&lcp.ID,
			&lcp.ChapterID,
			&lcp.Page,
		)

		ua.addLogger(logger{
			tables: []logtable{lComicPage},
			fn:     lcp.log,
		})
		return err
	})
}

func (dupe *Dupe) removeInferior(id int) {
	var newInf []*Post

	for _, inf := range dupe.Inferior {
		if inf.ID != id {
			newInf = append(newInf, inf)
		}
	}

	dupe.Inferior = newInf
}

// Ensures non of the inferiors are inferior of other dupe sets
func (dupe *Dupe) conflicts(tx *sql.Tx) error {
	var conflict DupeConflict

	// there is a conflict if an inferior already is an inferior of another dupe
	// the superior of the other dupe needs to be check against the superior of
	// this dupe
	err := query.Rows(
		tx,
		fmt.Sprintf(`
			SELECT post_id, dup_id
			FROM duplicates
			WHERE dup_id IN(%s)
			`,
			sep(",", len(dupe.Inferior), dupe.strindex),
		),
	)(func(scan scanner) error {
		var sup, inf int
		err := scan(&sup, &inf)

		// remove inferiors already assigned to superior
		if sup == dupe.Post.ID {
			dupe.removeInferior(inf)
		} else {
			conflict.check = append(conflict.check, sup)
		}

		return err
	})
	if err != nil {
		return err
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
