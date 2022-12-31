package migrate

import (
	"errors"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/set"
)

type (
	// Program is run after a migration file has been executed
	Program func(q ExecQuery) error

	// Migrator applies schema upgrades to the database
	Migrator struct {
		applied  set.Ordered[fileIdentifier]
		files    files
		queue    fileQueue
		programs programs
	}

	files    map[fileIdentifier]file
	programs map[fileIdentifier]Program
)

// Recursivly read migration files from directory
func FromDir(dir string) (Migrator, error) {
	var mig = Migrator{
		files:    make(files),
		programs: make(programs),
	}

	return mig, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		var (
			filename = entry.Name()
			ext      = filepath.Ext(filename)
		)

		if err != nil || entry.IsDir() || ext != ".sql" {
			return err
		}

		filename = strings.TrimSuffix(filename, ext)

		if _, ok := mig.files[fileIdentifier(filename)]; ok {
			return ErrDuplicateEntry(filename + ext)
		}

		mig.files[fileIdentifier(filename)], err = parseFile(path)

		return err
	})
}

var migratorCheckpoint = []string{
	"counter",
	"mimes",
	"roots",
	"users",
	"namespaces",
	"posts",
	"tombstone",
	"apple_tree",
	"tags",
	"post_tag_mappings",
	"duplicates",
	"reports",
	"spine",
	"messages",
	"parents",
	"comics",
	"log_comics",
	"log_post_tags",
	"wall",
	"log_duplicates",
	"score",
	"search_count_cache",
	"dns",
	"log_tags",
	"log_post",
	"phash",
	"duplicate_reports",
	"thumbnails",
	"views",
	"alias",
	"pools",
}

// Initialize the migration table in the database
func (mig Migrator) Initialize(q ExecQuery) error {
	var exists bool
	err := q.QueryRow(
		`SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE tablename = 'schema_migrations'
		)`,
	).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		_, err = q.Exec(
			`CREATE TABLE schema_migrations(
				applied TEXT NOT NULL,
				timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
		)
		if err != nil {
			return err
		}
	}

	// Check if this is an upgrade from the old system
	err = q.QueryRow(
		`SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE tablename = 'dbinfo'
		)`,
	).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Check which db version
		var ver int
		err = q.QueryRow(
			`SELECT ver
			FROM dbinfo`,
		).Scan(&ver)
		if err != nil {
			return err
		}

		if ver != 25 {
			return errors.New("old database version: please check out git tag 'old-migration' and apply its migrations first")
		}

		// Apply the new migrations
		for _, table := range migratorCheckpoint {
			_, err = q.Exec(
				`INSERT INTO schema_migrations(applied)
				VALUES($1)`,
				table,
			)
			if err != nil {
				return err
			}
		}

		// Remove dbinfo
		_, err = q.Exec("DROP TABLE dbinfo")
		if err != nil {
			return err
		}
	}

	return nil
}

func (mig *Migrator) InstallProgram(fid fileIdentifier, program Program) {
	mig.programs[fid] = program
}

func (mig *Migrator) FetchApplied(q ExecQuery) error {
	return db.QueryRows(
		q,
		`SELECT applied
		FROM schema_migrations`,
	)(func(scan db.Scanner) error {
		var fileID fileIdentifier
		err := scan(&fileID)
		mig.applied.Set(fileID)
		return err
	})
}

func (mig *Migrator) EnqueueMigrations() error {
	for fid := range mig.files {
		err := mig.enqueueFile(fid)
		if err != nil {
			return err
		}
	}

	return nil
}

// True if there are migrations left to run
func (mig *Migrator) Next() bool {
	return mig.queue.next()
}

// Executes the migration
func (mig *Migrator) Execute(q ExecQuery) error {
	fid := mig.queue.dequeue()
	log.Printf("applying migration '%s.sql'\n", fid)
	return mig.executeFile(q, fid)
}

func (mig *Migrator) enqueueFile(fid fileIdentifier) error {
	if _, ok := mig.files[fid]; !ok {
		return ErrDependencyMissing(fid)
	}

	// Already in queue
	if mig.queue.has(fid) {
		return nil
	}

	// Already applied migration
	if mig.applied.Has(fid) {
		return nil
	}

	// Make sure dependencies are enqueued first
	for _, dependency := range mig.files[fid].dependencies {
		if err := mig.enqueueFile(dependency); err != nil {
			return err
		}
	}

	mig.queue.enqueue(fid)
	return nil
}

func (mig *Migrator) executeFile(q ExecQuery, fid fileIdentifier) error {
	mig.applied.Set(fid)

	file := mig.files[fid]

	_, err := q.Exec(file.sql)
	if err != nil {
		return ErrFailedMigration{fid, err}
	}

	_, err = q.Exec(
		`INSERT INTO schema_migrations(applied)
		VALUES($1)`,
		fid,
	)
	if err != nil {
		return err
	}

	// Run program
	if prog, ok := mig.programs[fid]; ok {
		return prog(q)
	}

	return nil
}
