package store

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	shell "github.com/ipfs/go-ipfs-api"
)

// Do not use
type quickStore struct {
	ipfs *shell.Shell
	root string
}

func readQuickRoot() string {
	root, err := ioutil.ReadFile("quick.root")
	if err != nil {
		log.Fatal(err)
		return ""
	}

	return string(root)
}

func NewQuickStore(shell *shell.Shell) *quickStore {
	return &quickStore{
		root: readQuickRoot(),
		ipfs: shell,
	}
}

func (s *quickStore) Store(cid, destination string) error {
	var (
		newRoot string
		err     error
	)

	if s.root == "" {
		s.root = cidV1UnixfsDir
	}

	newRoot, err = s.ipfs.PatchLink(s.root, destination, cid, true)
	if err != nil {
		return err
	}

	s.root = newRoot

	return s.store()
}

func (s *quickStore) Remove(src string) error {
	return nil
}

func (s *quickStore) store() error {
	f, err := os.OpenFile("quick.root", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, s.root)
	return err
}
