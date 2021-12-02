package DataManager

import (
	"database/sql"
)

// Add and remove tags from a post
// performing related parent/alias lookups
// and producing a log
func AlterPostTags(postID int, tagstr, tagdiff string) loggingAction {
	return func(tx *sql.Tx) (Logger, error) {
		tags, err := parseTags(tagstr, '\n')
		if err != nil {
			return nil, err
		}

		err = tags.save(tx)
		if err != nil {
			return nil, err
		}

		diff, err := parseTags(tagdiff, '\n')
		if err != nil {
			return nil, err
		}

		tags, err = tags.upgrade(tx)
		if err != nil {
			return nil, err
		}

		// Get tags in db
		in, err := postTags(tx, postID)
		if err != nil {
			return nil, err
		}

		var (
			add    = tags.diff(diff)
			remove = diff.diff(tags)
		)

		// Reduce to only add new tags
		add = add.diff(in)

		// Reduce to only remove existing tags
		remove = in.diff(in.diff(remove))

		// Nothing to do, return nil
		if len(add)+len(remove) <= 0 {
			return nil, nil
		}

		err = prepPTExec(tx, queryDeletePTM, postID, remove)
		if err != nil {
			return nil, err
		}

		err = prepPTExec(tx, queryInsertPTM, postID, add)
		if err != nil {
			return nil, err
		}

		return logPostTags{
			PostID:  postID,
			Added:   add,
			Removed: remove,
		}, nil

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
