package store

type Storage interface {
	// Store a cid in destination
	Store(cid, destination string) error
	// Remove file
	Remove(src string) error
}

type NullStorage struct{}

func (n *NullStorage) Store(cid, destination string) error { return nil }
func (n *NullStorage) Remove(src string) error             { return nil }
