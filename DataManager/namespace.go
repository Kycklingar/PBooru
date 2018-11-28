package DataManager

import (
	"database/sql"
	"errors"
	"log"
	"strings"

	C "github.com/kycklingar/PBooru/DataManager/cache"
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

	if t := C.Cache.Get("NSP", n.Namespace); t != nil {
		if ns, ok := t.(*Namespace); ok {
			//fmt.Println("Cache get:", ns.Namespace)
			*n = *ns
			if n.ID != 0 {
				return n.ID
			}
		}
	}
	//fmt.Println("Cache miss:", n.Namespace)

	err := q.QueryRow("SELECT id FROM namespaces WHERE nspace=$1", n.Namespace).Scan(&n.ID)
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

	err := q.QueryRow("SELECT nspace FROM namespaces WHERE id=$1", n.ID).Scan(&n.Namespace)
	if err != nil {
		log.Print(err)
		return ""
	}

	if t := C.Cache.Get("NSP", n.Namespace); t != nil {
		if ns, ok := t.(*Namespace); ok {
			*n = *ns
		}
	} else {
		C.Cache.Set("NSP", n.Namespace, n)
	}

	return n.Namespace
}

func (n *Namespace) GetCache() bool {
	if n.Namespace == "" {
		return false
	}
	if t := C.Cache.Get("NSP", n.Namespace); t != nil {
		if ns, ok := t.(*Namespace); ok {
			*n = *ns
			return true
		}
	}
	return false
}

func (n *Namespace) SetCache() {
	if n.GetCache() {
		return
	}
	if n.Namespace == "" {
		//fmt.Println("Empty namespace cache")
		return
	}
	C.Cache.Set("NSP", n.Namespace, n)
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

	err := q.QueryRow("INSERT INTO namespaces(nspace) VALUES($1) RETURNING id", n.Namespace).Scan(&n.ID)
	if err != nil {
		return err
	}

	return nil
}
