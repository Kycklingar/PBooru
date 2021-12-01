package DataManager

import "database/sql"

func noRows(err error) error {
	if err == sql.ErrNoRows {
		return nil
	}

	return err
}
