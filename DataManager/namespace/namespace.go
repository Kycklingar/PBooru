package namespace

import (
	"database/sql"
	"sync"
)

type Namespace string

type rowQuery interface {
	QueryRow(string, ...any) *sql.Row
}

// namespaces hold all namespaces and their id
var (
	mut        sync.RWMutex
	namespaces = map[Namespace]int{}
)

// New will return the namespace id, creating it if necessary
func New(db rowQuery, namespaceStr string) (Namespace, int, error) {
	var namespace = Namespace(namespaceStr)

	id, err := namespace.ID(db)
	if err == nil || err != sql.ErrNoRows {
		return namespace, id, err
	}

	id, err = insert(db, namespace)
	return namespace, id, err
}

// ID returns the namespace, querying the database if necessary
func (n Namespace) ID(db rowQuery) (int, error) {
	id, ok := readFromMap(n)
	if ok {
		return id, nil
	}

	return query(db, n)
}

func insert(db rowQuery, namespace Namespace) (int, error) {
	return queryIntoMap(
		db,
		`INSERT INTO namespaces(nspace)
		VALUES($1)
		RETURNING id`,
		namespace,
	)
}

func query(db rowQuery, namespace Namespace) (int, error) {
	return queryIntoMap(
		db,
		`SELECT id
		FROM namespaces
		WHERE nspace = $1`,
		namespace,
	)
}

func queryIntoMap(db rowQuery, q string, namespace Namespace) (int, error) {
	var (
		id  int
		err error
	)
	err = db.QueryRow(q, namespace).Scan(&id)
	if err != nil {
		return 0, err
	}

	mut.Lock()
	defer mut.Unlock()

	namespaces[namespace] = id
	return id, err
}

func readFromMap(namespace Namespace) (int, bool) {
	mut.RLock()
	defer mut.RUnlock()
	id, ok := namespaces[namespace]
	return id, ok
}
