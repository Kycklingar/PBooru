package DataManager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/kycklingar/PBooru/DataManager/db"
	migrate "github.com/kycklingar/PBooru/DataManager/migrator"
	st "github.com/kycklingar/PBooru/DataManager/storage"
	"github.com/kycklingar/PBooru/DataManager/user"
	"github.com/kycklingar/PBooru/DataManager/user/flag"
	_ "github.com/lib/pq"
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

	err = DB.Ping()
	if err != nil {
		panic(err)
	}

	db.Context = DB

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

	// Setup session keeper
	user.InitSession(DB)

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

// Run database migrations
func update(db *sql.DB, dir string) error {
	migrator, err := migrate.FromDir(dir)
	if err != nil {
		return err
	}

	err = migrator.Initialize(db)
	if err != nil {
		return err
	}

	err = migrator.FetchApplied(db)
	if err != nil {
		return err
	}

	migrator.InstallProgram("users", createAdminAccount)

	err = migrator.EnqueueMigrations()
	if err != nil {
		return err
	}

	for migrator.Next() {
		tx, err := db.Begin()
		if err = migrator.Execute(tx); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func createAdminAccount(q migrate.ExecQuery) error {
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

	return user.Register(context.Background(), "admin", password, flag.All)
}

type Config struct {
	//Database string
	ConnectionString string
	StdUserFlag      flag.Flag
	Store            string
	MFSRootDir       string
	ThumbnailFormat  string
	ThumbnailSizes   []uint
	ThumbnailQuality int
}

func (c *Config) Default() {
	c.ConnectionString = "user=pbdb dbname=pbdb sslmode=disable"
	c.Store = "pin"
	c.MFSRootDir = "/pbooru/"
	c.ThumbnailFormat = "JPEG"
	c.ThumbnailSizes = []uint{1024, 512, 256}
	c.ThumbnailQuality = 90
	c.StdUserFlag = flag.Tagging | flag.Upload
}

var CFG *Config
