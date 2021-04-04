package store

import (
	"context"
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
)

const cidV1UnixfsDir = "bafybeiczsscdsbs7ffqz55asqdf3smv6klcw3gofszvwlyarci47bgf354"

func NewPinstore(ipfs *shell.Shell, rooter Rooter) (*pinstore, error) {
	return &pinstore{
		ipfs:   ipfs,
		rooter: rooter,
	}, nil
}

type pinstore struct {
	ipfs   *shell.Shell
	rooter Rooter
}

func (s *pinstore) Store(cid, destination string) error {
	var (
		root    string
		newRoot string
		err     error
	)

	err = s.rooter.Lock()
	if err != nil {
		return err
	}
	defer s.rooter.Unlock(&err)

	root, err = s.rooter.Root()
	if err != nil {
		return err
	}

	if root == "" {
		root = cidV1UnixfsDir
	}

	newRoot, err = s.ipfs.PatchLink(root, destination, cid, true)
	if err != nil {
		return err
	}

	err = s.rooter.UpdateRoot(newRoot)
	if err != nil {
		return err
	}

	err = s.updatePin(root, newRoot)

	return err
}

func (s *pinstore) Remove(target string) error {
	var (
		oldRoot string
		newRoot string
		err     error
	)

	err = s.rooter.Lock()
	if err != nil {
		return err
	}
	defer s.rooter.Unlock(&err)

	oldRoot, err = s.rooter.Root()
	if err != nil {
		return err
	}

	newRoot, err = s.ipfs.Patch(oldRoot, "rm-link", target)
	if err != nil {
		return err
	}

	err = s.updatePin(oldRoot, newRoot)

	return err
}

func (s *pinstore) updatePin(oldRoot, newRoot string) error {
	// Try updating old root
	if oldRoot != "" {
		// Check if oldRoot == patcher.Root
		// There is a bug in IPFS where updating pinA with pinA even if pinA isn't pinned yields no error
		if oldRoot == newRoot {
			// If pinned, return
			err := s.ipfs.Request("pin/ls", oldRoot).Option("type", "recursive").Exec(context.Background(), nil)
			if err == nil {
				return nil
			}
		} else {
			rb := s.ipfs.Request("pin/update", oldRoot, newRoot)
			err := rb.Exec(context.Background(), nil)
			// Some new pin check in IPFS
			// Destroy old pin and return operation
			if err != nil && err.Error() == "pin/update: 'to' cid was already recursively pinned" {
				err = s.ipfs.Unpin(oldRoot)
			}
			// Return if no error OR error is other than 'not pinned already'
			// Otherwise fall back to creating new pin
			if err == nil || (err != nil && err.Error() != "pin/update: 'from' cid was not recursively pinned already") {
				return err
			}
		}
	}

	// Pin not found, create new
	fmt.Println("Creating new pin: ", newRoot)
	return s.ipfs.Pin(newRoot)
}
