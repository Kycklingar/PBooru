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
			add    = tagSetDiff(tags, diff)
			remove = tagSetDiff(diff, tags)
		)

		// Reduce to only add new tags
		add = tagSetDiff(add, in)

		// Reduce to only remove existing tags
		remove = tagSetDiff(in, tagSetDiff(in, remove))

		// Nothing to do, return nil
		if len(add)+len(remove) <= 0 {
			return nil, nil
		}

		prepped := func(query string, set tagSet) error {
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

		err = prepped(`
			DELETE FROM post_tag_mappings
			WHERE post_id = $1
			AND tag_id = $2
			`,
			remove,
		)
		if err != nil {
			return nil, err
		}

		err = prepped(`
			INSERT INTO post_tag_mappings (
				post_id,
				tag_id
			)
			VALUES($1, $2)
			`,
			add,
		)
		if err != nil {
			return nil, err
		}

		return logPostTags{
			postID:  postID,
			Added:   add,
			Removed: remove,
		}, nil

	}
}
