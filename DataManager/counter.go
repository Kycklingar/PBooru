package DataManager

import (
	"database/sql"
	"log"
	"time"
)

func Counter() int {
	count++
	return count
}

var count int
var last int

func updateCounter() {
	defer time.AfterFunc(time.Second*10, updateCounter)
	if count == 0 {
		err := DB.QueryRow("SELECT count FROM counter").Scan(&last)
		if err != nil && err == sql.ErrNoRows {
			_, err = DB.Exec("INSERT INTO counter VALUES (0)")
			if err != nil {
				log.Print(err)
				return
			}
		} else if err != nil {
			log.Print(err)
			return
		}
		count = last
	}
	if last < count {
		if _, err := DB.Exec("UPDATE counter SET count=?", count); err != nil {
			log.Print(err)
		}

		last = count
	}
}
