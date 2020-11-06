package store

type Rooter interface {
	Lock() error
	Unlock(*error) error

	Root() (string, error)
	UpdateRoot(string) error
}
