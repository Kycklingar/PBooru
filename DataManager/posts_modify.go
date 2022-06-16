package DataManager

import (
	"database/sql"
	"log"
)

func PostAddMetaData(postID int, metaStr string) []loggingAction {
	var acts []loggingAction

	for _, md := range parseMetaDataString(metaStr) {
		if md.Namespace() == "date" {
			acts = append(acts, postAddCreationDate(postID, md))
		} else {
			acts = append(acts, postAddMetaData(postID, md))
		}
	}

	return acts
}

func PostRemoveMetaData(postID int, metaDataStrings []string) []loggingAction {
	var acts []loggingAction

	for _, mds := range metaDataStrings {
		if md := parseMetaData(mds); md != nil {
			if md.Namespace() == "date" {
				acts = append(acts, postRemoveCreationDate(postID, md))
			} else {
				acts = append(acts, postRemoveMetaData(postID, md))
			}
		}
	}

	return acts

}

func postAddCreationDate(postID int, md MetaData) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		res, err := tx.Exec(`
			INSERT INTO post_creation_dates (
				post_id,
				created
			)
			VALUES($1, $2)
			ON CONFLICT DO NOTHING
			`,
			postID,
			md.value(),
		)
		if err != nil {
			log.Println(err)
			return
		}

		ra, err := res.RowsAffected()
		if err != nil || ra <= 0 {
			return
		}

		err = updatePostCreationDate(postID, tx)

		l.addTable(lPostCreationDates)
		l.fn = logPostCreationDates{
			postID: postID,
			Action: aCreate,
			Date:   md.(metaDate),
		}.log

		return
	}
}

func postRemoveCreationDate(postID int, md MetaData) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		_, err = tx.Exec(`
			DELETE FROM post_creation_dates
			WHERE post_id = $1
			AND created = $2
			`,
			postID,
			md.value(),
		)
		if err != nil {
			log.Println(err)
			return
		}

		err = updatePostCreationDate(postID, tx)

		l.addTable(lPostCreationDates)
		l.fn = logPostCreationDates{
			postID: postID,
			Action: aDelete,
			Date:   md.(metaDate),
		}.log

		return
	}
}

func updatePostCreationDate(postID int, tx *sql.Tx) error {
	_, err := tx.Exec(`
		UPDATE posts
		SET creation_date = (
			SELECT MIN(created)
			FROM post_creation_dates
			WHERE post_id = $1
		)
		`,
		postID,
	)

	return err
}

func postAddMetaData(postID int, md MetaData) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		res, err := tx.Exec(`
			INSERT INTO post_metadata (
				post_id,
				namespace,
				metadata
			)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING
			`,
			postID,
			md.Namespace(),
			md.value(),
		)
		if err != nil {
			return
		}

		ra, err := res.RowsAffected()
		if err != nil || ra <= 0 {
			return
		}

		l.addTable(lPostMetaData)
		l.fn = logPostMetaData{
			PostID:    postID,
			Action:    aCreate,
			Namespace: md.Namespace(),
			MetaData:  md.Data(),
		}.log
		return
	}
}

func postRemoveMetaData(postID int, md MetaData) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		_, err = tx.Exec(`
			DELETE FROM post_metadata
			WHERE post_id = $1
			AND namespace = $2
			AND metadata = $3
			`,
			postID,
			md.Namespace(),
			md.value(),
		)

		l.addTable(lPostMetaData)
		l.fn = logPostMetaData{
			PostID:    postID,
			Action:    aDelete,
			Namespace: md.Namespace(),
			MetaData:  md.Data(),
		}.log
		return
	}
}

func PostChangeDescription(postID int, newDescr string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		_, err = tx.Exec(`
			UPDATE posts
			SET description = $1
			WHERE id = $2
			`,
			newDescr,
			postID,
		)

		l.addTable(lPostDescription)
		l.fn = logPostDescription{
			PostID:      postID,
			Description: newDescr,
		}.log
		return
	}
}
