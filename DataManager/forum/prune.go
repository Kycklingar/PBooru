package forum

import "database/sql"

// Delete threads > board limit
// If the board is of type archival lock thread
func PruneBoard(tx *sql.Tx, board string) error {
	var (
		//cycle bool
		threadLimit int
	)

	err := tx.QueryRow(`
		SELECT thread_limit
		FROM forum_board
		WHERE uri = $1
		`,
		board,
	).Scan(&threadLimit)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM forum_thread tl
		USING (
			SELECT id
			FROM forum_thread
			WHERE board = $1
			ORDER BY bumped DESC
			OFFSET $2
		) as tr
		WHERE tl.id = tr.id
		`,
		board,
		threadLimit,
	)
	return err
}
