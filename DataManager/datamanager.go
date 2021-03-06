package DataManager

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"strings"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	st "github.com/kycklingar/PBooru/DataManager/storage"
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
	Prepare(string) (*sql.Stmt, error)
}

func txError(tx *sql.Tx, err error) error {
	tx.Rollback()
	return err
}

var DB *sql.DB

var store st.Storage

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
	if err = cacheAllMimes(); err != nil {
		log.Println(err)
	}

	rand.Seed(time.Now().UnixNano())

	ipfs = shell.NewShell(iApi)
	if !ipfs.IsUp() {
		log.Printf("Your IPFS Daemon is not accessible on %s. Did you forget to start it?\n", iApi)
	}

	switch CFG.Store {
	case "pin":
		store, err = st.NewPinstore(ipfs, st.NewPgRooter("", DB))
		if err != nil {
			log.Fatal(err)
		}
	case "mfs":
		store = st.NewMfsStore(CFG.MFSRootDir, ipfs)
	case "quick":
		store = st.NewQuickStore(ipfs)
	default:
		store = new(st.NullStorage)
	}

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
			u.flag = new(flag)
			*u.flag = flag(flag(flagAll))
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

			_, err = tx.Exec("INSERT INTO users(username, passwordhash, salt, datejoined, adminflag) VALUES($1, $2, $3, CURRENT_TIMESTAMP, $4)", u.Name, u.passwordHash, u.salt, u.Flag())
			if err != nil {
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

type Config struct {
	//Database string
	ConnectionString string
	StdUserFlag      int
	Store            string
	MFSRootDir       string
	ThumbnailFormat  string
	ThumbnailSizes   []int
	ThumbnailQuality int
}

func (c *Config) Default() {
	c.ConnectionString = "user=pbdb dbname=pbdb sslmode=disable"
	c.Store = "pin"
	c.MFSRootDir = "/pbooru/"
	c.ThumbnailFormat = "JPEG"
	c.ThumbnailSizes = []int{1024, 512, 256}
	c.ThumbnailQuality = 90
	c.StdUserFlag = flagTagging | flagUpload
}

var CFG *Config
