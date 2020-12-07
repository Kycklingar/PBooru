package forum

import "database/sql"

func Bump(tx *sql.Tx, thread int) error {
	_, err := tx.Exec(`
		UPDATE forum_thread
		SET bump_count = bump_count + 1,
		bumped = CURRENT_TIMESTAMP
		WHERE id = $1
		`,
		thread,
	)

	return err
}
