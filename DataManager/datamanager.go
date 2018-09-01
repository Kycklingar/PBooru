package DataManager

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

const (
	sqlite3Timestamp  = "2006-01-02 15:04:05"
	Sqlite3Timestamp  = "2006-01-02 15:04:05"
	Fsqlite3Timestamp = "2006-01-02T15:04:05Z"
)

type querier interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func txError(tx *sql.Tx, err error) error {
	tx.Rollback()
	return err
}

var DB *sql.DB

func Setup(iApi string) {
	var err error
	DB, err = sql.Open("sqlite3", "PBDB.db")
	if err != nil {
		panic(err)
	}
	if DB == nil {
		panic(err)
	}
	err = DB.Ping()
	if err != nil {
		panic(err)
	}

	_, err = DB.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	_, err = DB.Exec("PRAGMA cache_size=250000")
	if err != nil {
		panic(err)
	}
	_, err = DB.Exec("PRAGMA busy_timeout=5000")
	if err != nil {
		panic(err)
	}

	DB.SetMaxOpenConns(1)

	err = update(DB, "sql")
	if err != nil {
		panic(err)
	}

	go sessionGC()
	go updateCounter()

	rand.Seed(time.Now().UnixNano())

	ipfsAPI = iApi

	// countCache.cache = make(map[string]int)
}

func Close() error {
	err := DB.Close()
	return err
}

func update(db *sql.DB, folder string) error {
	files, _ := ioutil.ReadDir(fmt.Sprintf("%s/", folder))
	num := len(files)

	dbVer := getDbVersion(db)

	for i := dbVer + 1; i < num+1; i++ {
		fmt.Println("Updating to version ", i)
		dat, err := ioutil.ReadFile(fmt.Sprintf("%s/up%d.sql", folder, i))
		if err != nil {
			return err
		}

		sqlString := string(dat)

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		_, err = tx.Exec(sqlString)
		if err != nil {
			tx.Rollback()
			return err
		}

		err = updateCode(i, tx)
		if err != nil {
			tx.Rollback()
			return err
		}

		err = setDbVersion(i, tx)
		if err != nil {
			return err
		}
		err = tx.Commit()
		if err != nil {
			return err
		}
		fmt.Println("Success")
	}
	return nil
}

func updateCode(ver int, tx *sql.Tx) error {
	switch ver {
	case 8:
		{
			var password string
			for {
				fmt.Print("Choose a password for the admin account:")
				var pass, pass2 string

				fmt.Scanln(&pass)
				fmt.Print("Confirm password:")
				fmt.Scanln(&pass2)

				if pass == pass2 {
					password = pass
					break
				}
				fmt.Println("Passwords do not match.")
			}

			u := NewUser()
			u.AdmFlag = AdmFAdmin
			u.Name = "admin"
			var err error
			u.salt, err = createSalt()
			if err != nil {
				return err
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(password+u.salt), 0)
			if err != nil {
				return err
			}
			u.passwordHash = string(hash)

			_, err = tx.Exec("INSERT INTO users(username, passwordhash, salt, datejoined, adminflag) VALUES($1, $2, $3, CURRENT_TIMESTAMP, $4)", u.Name, u.passwordHash, u.salt, u.AdmFlag)
			if err != nil {
				return err
			}
		}
	// case 14:
	// 	rows, err := tx.Query("SELECT id, multihash FROM posts WHERE thumbhash='NO THUMBNAIL'")
	// 	if err != nil {
	// 		return err
	// 	}
	// 	defer rows.Close()

	// 	for i := 0; rows.Next(); i++ {
	// 		if i > 10 {
	// 			fmt.Println("Checkpoint")
	// 			i = 0
	// 			tx.Commit()
	// 			tx, err = db.Begin()
	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 		}
	// 		var id int
	// 		var multihash string
	// 		err = rows.Scan(&id, &multihash)
	// 		if err != nil {
	// 			return err
	// 		}
	// 		fmt.Println("Working on post ", id, " ", multihash)

	// 		file := ipfsCat(multihash)
	// 		if file == nil {
	// 			panic("file is nil")
	// 		}
	// 		defer file.Close()

	// 		thumbhash, err := makeThumbnail(file)
	// 		if err != nil {
	// 			fmt.Println(err)
	// 			continue
	// 		}
	// 		fmt.Println("Got thumbnailhash: ", thumbhash)
	// 		_, err = tx.Exec("UPDATE posts SET thumbhash=$1 WHERE id=$2", thumbhash, id)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// 	err = rows.Err()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	break
	case 21:
		var height *int
		err := tx.QueryRow("SELECT MAX(post_id) FROM phash").Scan(&height)
		if err != nil && err != sql.ErrNoRows {
			log.Println(err)
			return err
		}
		if height == nil {
			return nil
		}
		for i := 0; ; i++ {
			rows, err := tx.Query("SELECT id, thumbhash FROM posts WHERE thumbhash !='NT' AND id > $2 LIMIT 1000 OFFSET $1", i*1000, *height)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil
				}
				return err
			}

			type PHS struct {
				id int
				h1 uint16
				h2 uint16
				h3 uint16
				h4 uint16
			}

			var phs []PHS

			n := 0

			for rows.Next() {
				n++
				var (
					id   int
					hash string
				)
				err = rows.Scan(&id, &hash)
				if err != nil {
					return err
				}

				f := ipfsCat(hash)

				dhash := dHash(f)
				if err = f.Close(); err != nil {
					return err
				}

				fmt.Println(id, hash, dhash)

				var ph PHS

				ph.id = id

				b := make([]byte, 8)
				binary.BigEndian.PutUint64(b, dhash)
				ph.h1 = uint16(b[1]) | uint16(b[0])<<8
				ph.h2 = uint16(b[3]) | uint16(b[2])<<8
				ph.h3 = uint16(b[5]) | uint16(b[4])<<8
				ph.h4 = uint16(b[7]) | uint16(b[6])<<8

				phs = append(phs, ph)
			}
			rows.Close()

			if n == 0 {
				return nil
			}

			for _, ph := range phs {
				_, err = tx.Exec("INSERT INTO phash (post_id, h1, h2, h3, h4) VALUES($1, $2, $3, $4, $5)", ph.id, ph.h1, ph.h2, ph.h3, ph.h4)
				if err != nil {
					return err
				}
			}

			err = tx.Commit()
			if err != nil {
				return err
			}

			tx, err = DB.Begin()
			if err != nil {
				log.Println(err)
				return err
			}

		}

	}
	return nil
}

func getDbVersion(db *sql.DB) int {
	var ver int
	err := db.QueryRow("SELECT ver FROM dbinfo").Scan(&ver)
	if err != nil {
		ver = 0
	}

	return ver
}

func setDbVersion(ver int, tx *sql.Tx) error {
	_, err := tx.Exec("UPDATE dbinfo SET ver=$1", ver)
	return err
}
