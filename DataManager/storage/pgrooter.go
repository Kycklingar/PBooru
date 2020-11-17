package store

import (
	"database/sql"
	"log"
	"sync"
)

func NewPgRooter(id string, db *sql.DB) *pgRooter {
	return &pgRooter{
		id: id,
		db: db,
	}
}

type pgRooter struct {
	db *sql.DB

	tx *sql.Tx
	id string

	mut sync.Mutex
}

func (r *pgRooter) Lock() error {
	r.mut.Lock()

	var err error

	r.tx, err = r.db.Begin()
	if err != nil {
		return err
	}

	return err
}

func (r *pgRooter) Unlock(commitErr *error) error {
	defer r.mut.Unlock()

	var err error

	if *commitErr != nil {
		log.Println(*commitErr)
		err = r.tx.Rollback()
	} else {
		err = r.tx.Commit()
	}

	r.tx = nil

	return err
}

func (r *pgRooter) Root() (string, error) {
	var root string

	err := r.tx.QueryRow(`
		SELECT cid
		FROM roots
		WHERE id = $1
		FOR UPDATE
		`,
		r.id,
	).Scan(&root)

	if err == sql.ErrNoRows {
		_, err = r.tx.Exec(`
			INSERT INTO roots(
				id,
				cid
			)
			VALUES ($1, $2)
			`,
			r.id,
			"",
		)
	}

	return root, err
}

func (r *pgRooter) UpdateRoot(newRoot string) error {
	_, err := r.tx.Exec(`
		UPDATE roots
		SET cid = $1
		WHERE id = $2
		`,
		newRoot,
		r.id,
	)

	return err
}
