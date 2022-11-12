package DataManager

import (
	"database/sql"
	"log"

	"github.com/kycklingar/PBooru/DataManager/query"
)

func commitOrDie(tx *sql.Tx, err *error) {
	var terr error
	if *err != nil {
		terr = tx.Rollback()
	} else {
		terr = tx.Commit()
	}

	if terr != nil {
		log.Println(err)
	}
}

type scanner = query.Scanner
