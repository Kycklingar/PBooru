package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
)

// Add and remove tags from a post
// performing related parent/alias lookups
// and producing a log
func AlterPostTags(postID int, tagstr, tagdiff string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		tags, err := parseTags(tagstr, '\n').chain().save(tx).unwrap()
		if err != nil {
			return
		}

		tags, err = tags.chain().upgrade(tx).unwrap()
		if err != nil {
			return
		}

		diff := parseTags(tagdiff, '\n')

		// Get tags in db
		in, err := postTags(tx, postID).unwrap()
		if err != nil {
			return
		}

		var (
			add    = tags.diff(diff)
			remove = diff.diff(tags)
		)

		// Reduce to only add new tags
		add = add.diff(in).unique()

		// Reduce to only remove existing tags
		remove = in.diff(in.diff(remove)).unique()

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

		qErr := []func(q querier) tagSetChain{
			add.chain().recount,
			remove.chain().recount,
			add.chain().purgeCountCache,
			remove.chain().purgeCountCache,
		}

		for _, f := range qErr {
			if err = f(tx).err; err != nil {
				return
			}
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
		add, err := parseTags(addStr, '\n').chain().save(tx).unwrap()
		if err != nil {
			return
		}

		remove, err := parseTags(remStr, '\n').chain().save(tx).unwrap()
		if err != nil {
			return
		}

		remove = remove.diff(add)

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
					strSep(pids, ","),
				),
				tx,
				aCreate,
				add.unique(),
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
					sep(",", len(add), add.strindex),
					strSep(pids, ","),
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
					strSep(pids, ","),
				),
				tx,
				aDelete,
				remove.unique(),
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
					sep(",", len(remove), remove.strindex),
					strSep(pids, ","),
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

func prepPTExec(tx querier, query string, postID int, set tagSet) error {
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
