package DataManager

import (
	"context"
	"database/sql"
	"sync"
	"fmt"

	patcher "github.com/kycklingar/PBooru/DataManager/ipfs-patcher"
)

func newPinstore(id string) (*pinstore, error) {
	var root string
	err := DB.QueryRow("SELECT cid FROM roots WHERE id = $1", id).Scan(&root)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return &pinstore{
		patcher: patcher.NewPatcher(ipfs, root),
		id:      id,
	}, nil
}

type pinstore struct {
	patcher *patcher.Patcher
	id      string

	mutex sync.Mutex
}

func (s *pinstore) Store(cid, destination string) error {
	err := s.patcher.Cp(destination, cid)
	if err != nil {
		return err
	}

	return s.updateRoot()
}

func (s *pinstore) Remove(src string) error {
	err := s.patcher.Rm(src)
	if err != nil {
		return err
	}

	return s.updateRoot()
}

func (s *pinstore) updateRoot() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var oldRoot string

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer commitOrDie(tx, &err)

	err = tx.QueryRow(`
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
	} else {
		_, err = tx.Exec(`
			UPDATE roots
			SET cid = $1
			WHERE id = $2
			`,
			s.patcher.Root(),
			s.id,
		)
		if err != nil {
			return err
		}
	}

	err = s.updatePin(oldRoot)
	return err
}

func (s *pinstore) updatePin(oldRoot string) error {
	// Try updating old root
	if oldRoot != "" {
		// Check if oldRoot == patcher.Root
		// There is a bug in IPFS where updating pinA with pinA even if pinA isn't pinned yields no error
		if oldRoot == s.patcher.Root() {
			// If pinned, return
			err := ipfs.Request("pin/ls", oldRoot).Option("type", "recursive").Exec(context.Background(), nil)
			if err == nil {
				return nil
			}
		} else {
			rb := ipfs.Request("pin/update", oldRoot, s.patcher.Root())
			err := rb.Exec(context.Background(), nil)
			if err == nil || (err != nil && err.Error() != "pin/update: 'from' cid was not recursively pinned already") {
				return err
			}
		}
	}

	// Pin not found, create new
	fmt.Println("Creating new pin: ", s.patcher.Root())
	return ipfs.Pin(s.patcher.Root())
}

