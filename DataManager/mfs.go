package DataManager

import (
	"context"
	"path"

	shell "github.com/ipfs/go-ipfs-api"
)

type mfsStore struct{}

func (s *mfsStore) Store(cid, dest string) error {
	ctx := context.Background()

	dir, _ := path.Split(dest)

	if _, err := ipfs.FilesLs(ctx, path.Join(CFG.MFSRootDir, dir)); err != nil {
		opts := []shell.FilesOpt{
			shell.FilesMkdir.CidVersion(1),
			shell.FilesMkdir.Parents(true),
		}
		ipfs.FilesMkdir(ctx, dir, opts...)
	}

	if _, err := ipfs.FilesLs(ctx, dest); err == nil {
		return nil
	}

	return ipfs.FilesCp(ctx, path.Join("/ipfs/", cid), dest)
}

func (s *mfsStore) Remove(src string) error {
	return ipfs.FilesRm(context.Background(), src, false)
}
