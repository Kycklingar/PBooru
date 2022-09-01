package DataManager

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kycklingar/set"
)

func (p Post) Tags() ([]Tag, error) {
	set, err := postTags(DB, p.ID).ts().unwrap()
	return tsTags(set), err
}

// Add and remove tags from a post
// performing related parent/alias lookups
// and producing a log
func AlterPostTags(postID int, tagstr, tagdiff string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		tags, err := tsChain(parseTags(tagstr)).
			save(tx).
			upgrade(tx).
			ts().
			unwrap()
		if err != nil {
			return
		}

		diff := parseTags(tagdiff)

		// Get tags postTags db
		postTags, err := postTags(tx, postID).ts().unwrap()
		if err != nil {
			return
		}

		var (
			add    = tsTags(set.Diff(set.Diff(tags, diff), postTags))
			remove = tsTags(set.Intersection(postTags, set.Diff(diff, tags)))
		)

		// Nothing to do, return nil
		if len(add)+len(remove) <= 0 {
			return
		}

		err = prepPTExec(tx, queryDeletePTM, postID, remove)
		if err != nil {
			return
		}

		err = prepPTExec(tx, queryInsertPTM, postID, add)
		if err != nil {
			return
		}

		err = tagChain(add).addCount(tx, 1).purgeCountCache(tx).err
		if err != nil {
			return
		}

		err = tagChain(remove).addCount(tx, -1).purgeCountCache(tx).err
		if err != nil {
			return
		}

		err = clearEmptySearchCountCache(tx)
		if err != nil {
			return
		}

		l.addTable(lPostTags)
		l.fn = logPostTags{
			PostID:  postID,
			Added:   add,
			Removed: remove,
		}.log
		return

	}
}

func AlterManyPostTags(pids []int, addStr, remStr string, delim rune) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		add, err := tsChain(parseTags(addStr)).
			save(tx).
			upgrade(tx).
			unwrap()
		if err != nil {
			return
		}

		remove, err := tsChain(parseTags(remStr)).
			save(tx).
			unwrap()
		if err != nil {
			return
		}

		// only remove what we are not adding
		remove = set.Diff(remove, add)

		if len(pids) <= 0 || (len(add) <= 0 && len(remove) <= 0) {
			err = errors.New("nothing to do")
			return
		}

		var (
			multilogs logMultipleTags
			mls       logMultipleTags
		)

		if len(add) > 0 {
			mls, err = multiLogStmtFromSet(
				fmt.Sprintf(`
				SELECT DISTINCT p1.post_id
				FROM post_tag_mappings p1
				LEFT JOIN post_tag_mappings p2
				ON p1.post_id = p2.post_id
				AND p2.tag_id = $1
				WHERE p1.post_id IN(%s)
				AND p2.post_id IS NULL
				`,
					join(",", pids),
				),
				tx,
				aCreate,
				add,
			)
			if err != nil {
				return
			}

			multilogs = append(multilogs, mls...)

			_, err = tx.Exec(
				fmt.Sprintf(`
				INSERT INTO post_tag_mappings (
					post_id,
					tag_id
				)
				SELECT DISTINCT (post_id), UNNEST(ARRAY[%s])
				FROM post_tag_mappings
				WHERE post_id IN(%s)
				ON CONFLICT DO NOTHING
				`,
					tSetStr(add),
					join(",", pids),
				),
			)
			if err != nil {
				return
			}

		}

		if len(remove) > 0 {
			mls, err = multiLogStmtFromSet(
				fmt.Sprintf(`
				SELECT post_id
				FROM post_tag_mappings
				WHERE tag_id = $1
				AND post_id IN(%s)
				`,
					join(",", pids),
				),
				tx,
				aDelete,
				remove,
			)
			if err != nil {
				return
			}

			multilogs = append(multilogs, mls...)

			_, err = tx.Exec(
				fmt.Sprintf(`
				DELETE FROM
				post_tag_mappings ptm
				USING (
					SELECT post_id, UNNEST(ARRAY[%s]) AS tag_id
					FROM post_tag_mappings
					WHERE post_id IN(%s)
				) AS del
				WHERE ptm.post_id = del.post_id
				AND ptm.tag_id = del.tag_id
				`,
					tSetStr(remove),
					join(",", pids),
				),
			)
			if err != nil {
				return
			}
		}

		if len(multilogs) > 0 {
			l.addTable(lMultiTags)
			l.fn = multilogs.log
		}

		return
	}
}

const (
	queryInsertPTM = `INSERT INTO post_tag_mappings (
				post_id,
				tag_id
			)
			VALUES($1, $2)`

	queryDeletePTM = `DELETE FROM post_tag_mappings
			WHERE post_id = $1
			AND tag_id = $2`
)

func prepPTExec(tx querier, query string, postID int, set set.Sorted[Tag]) error {
	if len(set) <= 0 {
		return nil
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range set {
		_, err = stmt.Exec(postID, tag.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func postsTags(tx querier, pids []int) tagSetChain {
	var chain tagSetChain
	chain.err = query(
		tx,
		fmt.Sprintf(
			`SELECT DISTINCT tag_id
			FROM post_tag_mappings
			WHERE post_id IN(%s)`,
			join(",", pids),
		),
	)(func(scan scanner) error {
		var t Tag
		err := scan(&t.ID)
		chain.set.Set(t)
		return err
	})

	return chain
}
