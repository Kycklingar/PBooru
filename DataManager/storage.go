package DataManager

import (
	"context"
	"database/sql"
	"sync"

	shell "github.com/ipfs/go-ipfs-api"
	patcher "github.com/kycklingar/PBooru/DataManager/ipfs-patcher"
)

func newStorage(id string) (*storage, error) {
	var root string
	err := DB.QueryRow("SELECT cid FROM roots WHERE id = $1", id).Scan(&root)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return &storage{
		patcher: patcher.NewPatcher(ipfs, root),
		id:      id,
	}, nil
}

type storage struct {
	patcher *patcher.Patcher
	id      string

	mutex sync.Mutex
}

func (s *storage) Store(cid, destination string) error {
	err := s.patcher.Cp(destination, cid)
	if err != nil {
		return err
	}

	return s.updateRoot()
}

func (s *storage) Remove(src string) error {
	err := s.patcher.Rm(src)
	if err != nil {
		return err
	}

	return s.updateRoot()
}

func (s *storage) updateRoot() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var oldRoot string

	err := DB.QueryRow(`
		SELECT cid
		FROM roots
		WHERE id = $1
		`,
		s.id,
	).Scan(&oldRoot)

	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}

		tx, err := DB.Begin()
		if err != nil {
			return err
		}
		defer commitOrDie(tx, &err)

		_, err = tx.Exec(`
			INSERT INTO roots(id, cid)
			VALUES($1, $2)
			`,
			s.id,
			s.patcher.Root(),
		)
		if err != nil {
			return err
		}

		err = ipfs.Pin(s.patcher.Root())
		return err
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer commitOrDie(tx, &err)

	_, err = tx.Exec(`
		UPDATE roots
		SET cid = $1
		WHERE id = $2
		`,
		s.patcher.Root(),
		s.id,
	)

	rb := ipfs.Request("pin/update", oldRoot, s.patcher.Root())
	var res *shell.Response
	res, err = rb.Send(context.Background())
	if err != nil {
		return err
	}
	res.Close()

	return err
}
