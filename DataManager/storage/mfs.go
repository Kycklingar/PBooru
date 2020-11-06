package store

import (
	"context"
	"path"

	shell "github.com/ipfs/go-ipfs-api"
)

func NewMfsStore(rootDir string, ipfs *shell.Shell) *mfsStore {
	return &mfsStore{
		rootDir: rootDir,
		ipfs: ipfs,
	}
}

type mfsStore struct {
	rootDir string
	ipfs *shell.Shell
}

func (s *mfsStore) Store(cid, dest string) error {
	ctx := context.Background()

	dir, _ := path.Split(dest)

	if _, err := s.ipfs.FilesLs(ctx, path.Join(s.rootDir, dir)); err != nil {
		opts := []shell.FilesOpt{
			shell.FilesMkdir.CidVersion(1),
			shell.FilesMkdir.Parents(true),
		}
		s.ipfs.FilesMkdir(ctx, dir, opts...)
	}

	if _, err := s.ipfs.FilesLs(ctx, dest); err == nil {
		return nil
	}

	return s.ipfs.FilesCp(ctx, path.Join("/ipfs/", cid), dest)
}

func (s *mfsStore) Remove(target string) error {
	return s.ipfs.FilesRm(context.Background(), target, false)
}
