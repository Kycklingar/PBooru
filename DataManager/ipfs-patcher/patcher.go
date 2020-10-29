package patcher

import (
	"sync"

	shell "github.com/ipfs/go-ipfs-api"
)

const cidV1UnixfsDir = "bafybeiczsscdsbs7ffqz55asqdf3smv6klcw3gofszvwlyarci47bgf354"

func NewPatcher(shell *shell.Shell, root string) *Patcher {
	if root == "" {
		root = cidV1UnixfsDir
	}

	return &Patcher{
		shell: shell,
		root:  root,
	}
}

type Patcher struct {
	shell *shell.Shell
	root  string

	mutex sync.Mutex
}

func (p *Patcher) Root() string { return p.root }

func (p *Patcher) Cp(dir, cid string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	newRoot, err := p.shell.PatchLink(p.root, dir, cid, true)
	if err != nil {
		return err
	}

	p.root = newRoot

	return nil
}

func (p *Patcher) Rm(dir string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	newRoot, err := p.shell.Patch(p.root, "rm-link", dir)
	if err != nil {
		return err
	}

	p.root = newRoot

	return nil
}
