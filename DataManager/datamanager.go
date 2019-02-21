package DataManager

import (
	"bytes"
	"database/sql"
	"fmt"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	sqlite3Timestamp    = "2006-01-02 15:04:05"
	postgresqlTimestamp = "2006-01-02T15:04:05.000000Z"
	Sqlite3Timestamp    = "2006-01-02 15:04:05"
	Fsqlite3Timestamp   = "2006-01-02T15:04:05Z"
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
	DB, err = sql.Open("postgres", CFG.ConnectionString)
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

	// _, err = DB.Exec("PRAGMA journal_mode=WAL")
	// if err != nil {
	// 	panic(err)
	// }
	// _, err = DB.Exec("PRAGMA cache_size=250000")
	// if err != nil {
	// 	panic(err)
	// }
	// _, err = DB.Exec("PRAGMA busy_timeout=5000")
	// if err != nil {
	// 	panic(err)
	// }

	// DB.SetMaxOpenConns(1)

	// _, err = DB.Exec("SET autocommit=0")
	// if err != nil {
	// 	panic(err)
	// }

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
		//	sqlStrings := strings.Split(sqlString, ";")

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		//	for _, str := range sqlStrings {
		//		if len(strings.TrimSpace(str)) <= 0 {
		//			continue
		//		}
		//		fmt.Println("Executing: ", str)
		//		_, err = tx.Exec(str)
		//		if err != nil {
		//			tx.Rollback()
		//		return err
		//	}
		//}

		if len(strings.TrimSpace(sqlString)) <= 0 {
			continue
		}
		fmt.Println("Executing: ", sqlString)
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
	case 1:
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
	case 6:
		{
			type p struct{
				id int
				hash string
			}
			query := func(q querier, offset int)([]p, error){
					limit := 50000
					rows, err := q.Query("SELECT id, multihash FROM posts WHERE file_size = 0 LIMIT $1 OFFSET $2", limit, limit *  offset)
					if err != nil{
						return nil, err
					}
					defer rows.Close()
					offset++

					var ids []p
					for rows.Next(){
						var id p
						err := rows.Scan(&id.id, &id.hash)
						if err != nil{
							return nil, err
						}
						ids = append(ids, id)
					}
					return ids, nil
				}
			offset := 0
			for {

				ids, err := query(tx, offset)
				if err != nil{
					return err
				}
				if len(ids) <= 0{
					break
				}
				offset++

				for _, id := range ids{
					fmt.Printf("Working on id %d hash %s -> ", id.id, id.hash)
					size := ipfsSize(id.hash)
					fmt.Println(size)
					if size <= 0{
						return errors.New("size returned was <= 0")
					}

					_, err := tx.Exec("UPDATE posts SET file_size = $1 WHERE id = $2", size, id.id)
					if err != nil{
						return err
					}
				}
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

func MigrateMfs() {
	type post struct {
		Hash string
		ID   int
	}
	query := func(str string, offset int) ([]post, error) {
		rows, err := DB.Query(str, offset*20000)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		defer rows.Close()

		var posts []post

		for rows.Next() {
			var p post
			rows.Scan(&p.Hash, &p.ID)
			posts = append(posts, p)
		}

		return posts, nil
	}

	var err error

	offset := 0
	defer mfsFlush(CFG.MFSRootDir)

	for {
		posts, err := query("SELECT multihash, id FROM posts ORDER BY id ASC LIMIT 20000 OFFSET $1", offset)
		if err != nil || len(posts) <= 0 {
			break
		}
		for _, post := range posts {
			fmt.Printf("Working on file: [%d] %s\n", post.ID, post.Hash)
			if err = mfsCP(CFG.MFSRootDir+"files/", post.Hash, false); err != nil {
				log.Fatal(err)
			}
		}
		offset++
		mfsFlush(CFG.MFSRootDir)
	}
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	offset = 0

	tquery := func(str string, offset int) ([]Thumb, error) {
		rows, err := DB.Query(str, offset*20000)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var thumbs []Thumb
		for rows.Next() {
			var t Thumb
			rows.Scan(&t.Hash, &t.Size)
			thumbs = append(thumbs, t)
		}
		return thumbs, rows.Err()
	}

	for {
		var thumbs []Thumb
		if thumbs, err = tquery("SELECT multihash, dimension FROM thumbnails ORDER BY post_id ASC LIMIT 20000 OFFSET $1", offset); err != nil || len(thumbs) <= 0 {
			break
		}
		for _, thumb := range thumbs {
			fmt.Println("Working on thumbnail:", thumb)
			if err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumb.Size, "/"), thumb.Hash, false); err != nil {
				log.Fatal(err)
			}
		}
		offset++
		mfsFlush(CFG.MFSRootDir)
	}

	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}
}

func GenerateThumbnails(size int) {

	type P struct {
		id   int
		hash string
	}
	query := func(tx *sql.Tx, offset int) []P {

		rows, err := tx.Query("SELECT p.multihash, p.id FROM posts p LEFT JOIN thumbnails t ON p.id = t.post_id AND t.dimension = $1 WHERE t.post_id IS NULL AND p.mime_id IN(SELECT id FROM mime_type WHERE type = 'image' OR mime = 'pdf' OR mime = 'zip') ORDER BY p.id ASC LIMIT 200 OFFSET $2", size, offset)
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
		defer rows.Close()

		var hashes []P

		for rows.Next() {
			var p P
			err = rows.Scan(&p.hash, &p.id)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
			hashes = append(hashes, p)
		}
		return hashes
	}

	var failed int
	for {
		var tx, err = DB.Begin()
		if err != nil {
			log.Fatal(err)
		}

		hashes := query(tx, failed)

		if len(hashes) <= 0 {
			break
		}

		for _, hash := range hashes {
			fmt.Println("Working on post: ", hash.id, hash.hash)
			file := ipfsCat(hash.hash)
			var b bytes.Buffer
			b.ReadFrom(file)
			f := bytes.NewReader(b.Bytes())
			thash, err := makeThumbnail(f, size)
			file.Close()
			if err != nil {
				log.Println(err, hash)
				failed++
				continue
			} else if thash == "" {
				log.Println("makeThumbnail did not produce a hash", hash)
				failed++
				continue
			}
			err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", size, "/"), thash, false)
			if err != nil {
				log.Println(err, thash)
				failed++
				continue
			}
			_, err = tx.Exec("INSERT INTO thumbnails(post_id, dimension, multihash) VALUES($1, $2, $3)", hash.id, size, thash)
			if err != nil {
				tx.Rollback()
				log.Fatal(err)
			}
		}
		mfsFlush(CFG.MFSRootDir)
		tx.Commit()
	}
}

type Config struct {
	//Database string
	ConnectionString string
	MFSRootDir       string
	ThumbnailFormat  string
	ThumbnailSizes   []int
}

func (c *Config) Default() {
	c.ConnectionString = "user=pbdb dbname=pbdb sslmode=disable"
	c.MFSRootDir = "/pbooru/"
	c.ThumbnailFormat = "JPEG"
	c.ThumbnailSizes = []int{1024, 512, 256}
}

var CFG *Config
