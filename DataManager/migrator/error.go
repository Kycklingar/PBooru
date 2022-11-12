package migrate

import "fmt"

type (
	ErrDependencyMissing string
	ErrDuplicateEntry    string
	ErrFailedMigration   struct {
		fid fileIdentifier
		err error
	}
)

func (e ErrDependencyMissing) Error() string {
	return fmt.Sprint("missing dependency: ", string(e))
}

func (e ErrDuplicateEntry) Error() string {
	return fmt.Sprintf("duplicate entry for '%s'", string(e))
}

func (e ErrFailedMigration) Error() string {
	return fmt.Sprintf("migration failed for '%s'\n\t%v", e.fid, e.err)
}

func (e ErrFailedMigration) Unwrap() error { return e.err }
