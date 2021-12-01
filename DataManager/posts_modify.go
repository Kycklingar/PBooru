package DataManager

import (
	"database/sql"
	"time"
)

func PostAddCreationDate(postID int, date time.Time) loggingAction {
	return func(tx *sql.Tx) (Logger, error) {
		_, err := tx.Exec(`
			INSERT INTO post_creation_dates (
				post_id,
				created
			)
			VALUES($1, $2)
			`,
			postID,
			date,
		)
		if err != nil {
			return nil, err
		}

		err = updatePostCreationDate(postID, tx)

		return logPostCreationDates{
			postID: postID,
			Action: aCreate,
			Date:   date,
		}, err
	}
}

func PostRemoveCreationDate(postID int, date time.Time) loggingAction {
	return func(tx *sql.Tx) (Logger, error) {
		_, err := tx.Exec(`
			DELETE FROM post_creation_dates
			WHERE post_id = $1
			AND created = $2
			`,
			postID,
			date,
		)
		if err != nil {
			return nil, err
		}

		err = updatePostCreationDate(postID, tx)

		return logPostCreationDates{
			postID: postID,
			Action: aDelete,
			Date:   date,
		}, err
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

func PostAddMetaData(postID int, namespace, data string) loggingAction {
	return func(tx *sql.Tx) (Logger, error) {
		_, err := tx.Exec(`
			INSERT INTO post_metadata (
				post_id,
				namespace,
				metadata
			)
			VALUES ($1, $2, $3)
			`,
			postID,
			namespace,
			data,
		)

		return logPostMetaData{
			PostID:    postID,
			Action:    aCreate,
			Namespace: namespace,
			MetaData:  data,
		}, err
	}
}

func PostRemoveMetaData(postID int, namespace, data string) loggingAction {
	return func(tx *sql.Tx) (Logger, error) {
		_, err := tx.Exec(`
			DELETE FROM post_metadata
			WHERE post_id = $1
			AND namespace = $2
			AND metadata = $3
			`,
			postID,
			namespace,
			data,
		)

		return logPostMetaData{
			PostID:    postID,
			Action:    aDelete,
			Namespace: namespace,
			MetaData:  data,
		}, err
	}
}

func PostChangeDescription(postID int, newDescr string) loggingAction {
	return func(tx *sql.Tx) (Logger, error) {
		_, err := tx.Exec(`
			UPDATE posts
			SET description = $1
			WHERE id = $2
			`,
			newDescr,
			postID,
		)

		return logPostDescription{
			PostID:      postID,
			Description: newDescr,
		}, err
	}
}
