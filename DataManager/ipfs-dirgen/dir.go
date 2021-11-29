package ipfsdir

import (
	"encoding/json"

	gocid "github.com/ipfs/go-cid"
	shell "github.com/ipfs/go-ipfs-api"
)

type Dir struct {
	Data struct {
		Root struct {
			Bytes string `json:"bytes"`
		} `json:"/"`
	} `json:"Data"`

	Links []link `json:"Links"`

	name    string
	subdirs []*Dir
}

type link struct {
	Hash  gocid.Cid
	Name  string
	Tsize uint64
}

func NewDir(name string) *Dir {
	var d = Dir{name: name}
	d.Data.Root.Bytes = "CAE"
	return &d
}

func (d *Dir) AddLink(name, cid string, size uint64) error {
	c, err := gocid.Decode(cid)
	if err != nil {
		return err
	}

	d.Links = append(d.Links, link{
		Name:  name,
		Hash:  c,
		Tsize: size,
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
		total += l.Tsize
	}

	b, err := json.Marshal(d)
	if err != nil {
		return "", total, err
	}

	cid, err := ipfs.DagPut(b, "dag-json", "dag-pb")
	return cid, total, err
}
