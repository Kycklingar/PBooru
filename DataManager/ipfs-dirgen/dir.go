package ipfsdir

import (
	"encoding/json"

	gocid "github.com/ipfs/go-cid"
	shell "github.com/ipfs/go-ipfs-api"
	format "github.com/ipfs/go-ipld-format"
)

type Dir struct {
	Data  string        `json:"data"`
	Links []format.Link `json:"links"`

	name    string
	subdirs []*Dir
}

func NewDir(name string) *Dir {
	return &Dir{Data: "CAE=", name: name}
}

func (d *Dir) AddLink(name, cid string, size uint64) error {
	c, err := gocid.Decode(cid)
	if err != nil {
		return err
	}

	d.Links = append(d.Links, format.Link{
		Name: name,
		Cid:  c,
		Size: size,
	})

	return nil
}

func (d *Dir) AddDirs(sub ...string) *Dir {
	if len(sub) <= 0 {
		return d
	}

	if len(sub) > 1 {
		if sub[0] == "" {
			return d.AddDirs(sub[1:]...)
		}

		for _, dir := range d.subdirs {
			if dir.name == sub[0] {
				return dir.AddDirs(sub[1:]...)
			}
		}

		return d.AddDir(NewDir(sub[0])).AddDirs(sub[1:]...)
	}
	if sub[0] == "" {
		return d
	}

	for _, dir := range d.subdirs {
		if dir.name == sub[0] {
			return dir
		}
	}

	return d.AddDir(NewDir(sub[0]))
}

func (d *Dir) AddDir(s *Dir) *Dir {
	d.subdirs = append(d.subdirs, s)
	return s
}

func (d *Dir) Put(ipfs *shell.Shell) (string, uint64, error) {
	var total uint64

	for _, s := range d.subdirs {
		cid, size, err := s.Put(ipfs)
		if err != nil {
			return "", size, err
		}

		if err = d.AddLink(s.name, cid, size); err != nil {
			return "", size, err
		}
	}

	for _, l := range d.Links {
		total += l.Size
	}

	b, err := json.Marshal(d)
	if err != nil {
		return "", total, err
	}

	cid, err := ipfs.DagPut(b, "json", "dag-pb")
	return cid, total, err
}
