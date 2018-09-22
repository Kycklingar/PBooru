package DataManager

import (
	"database/sql"
	"errors"
	"log"
	"strings"
)

func NewNamespace() *Namespace {
	return &Namespace{}
}

type Namespace struct {
	ID        int
	Namespace string
}

func (n *Namespace) QID(q querier) int {
	if n.ID != 0 {
		return n.ID
	}
	if n.Namespace == "" {
		return 0
	}
	err := q.QueryRow("SELECT id FROM namespaces WHERE nspace=?", n.Namespace).Scan(&n.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return 0
	}
	return n.ID
}

func (n *Namespace) QNamespace(q querier) string {
	if n.Namespace != "" {
		return n.Namespace
	}
	if n.ID == 0 {
		return ""
	}

	err := q.QueryRow("SELECT nspace FROM namespaces WHERE id=?", n.ID).Scan(&n.Namespace)
	if err != nil {
		log.Print(err)
		return ""
	}
	return n.Namespace
}

func (n *Namespace) Save(q querier) error {
	if n.Namespace == "" {
		return errors.New("namespace: save: not enough arguments")
	}
	if n.QID(q) != 0 {
		return nil
	}

	if strings.ContainsAny(n.Namespace, ",") {
		return errors.New("Namespace cannot contain ','")
	}

	res, err := q.Exec("INSERT INTO namespaces(nspace) VALUES(?)", n.Namespace)
	if err != nil {
		return err
	}
	id64, err := res.LastInsertId()
	if err != nil {
		return err
	}
	n.ID = int(id64)

	return nil
}
