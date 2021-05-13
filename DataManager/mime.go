package DataManager

import (
	"errors"
	"log"
	"strings"
)

var Mimes []*Mime

func MimeIDsFromType(mslice []string) []int {
	var mimeIDs []int
	for _, mtype := range mslice {
		for _, mime := range Mimes {
			if mtype == mime.Type {
				mimeIDs = append(mimeIDs, mime.ID)
			}
		}
	}
	return mimeIDs
}

func cacheAllMimes() error {
	rows, err := DB.Query("SELECT id, mime, type FROM mime_type")
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var m = NewMime()
		err = rows.Scan(&m.ID, &m.Name, &m.Type)
		if err != nil {
			log.Println(err)
			return err
		}
		Mimes = append(Mimes, m)
	}

	return rows.Err()
}

func NewMime() *Mime {
	return &Mime{}
}

type Mime struct {
	ID   int
	Name string
	Type string
}

func (m *Mime) QID(q querier) int {
	if m.ID != 0 {
		return m.ID
	}

	if m.Name == "" || m.Type == "" {
		return 0
	}

	err := q.QueryRow("SELECT id FROM mime_type WHERE mime=$1 AND type=$2", m.Name, m.Type).Scan(&m.ID)
	if err != nil {
		log.Print(err)
	}

	return m.ID
}

func (m *Mime) QName(q querier) string {
	if m.Name != "" {
		return m.Name
	}

	if m.QID(q) == 0 {
		return ""
	}

	err := q.QueryRow("SELECT mime, type FROM mime_type WHERE id=$1", m.QID(q)).Scan(&m.Name, &m.Type)
	if err != nil {
		log.Print(err)
	}

	return m.Name
}

func (m *Mime) QType(q querier) string {
	if m.Type != "" {
		return m.Type
	}

	if m.QID(q) == 0 {
		return ""
	}

	err := q.QueryRow("SELECT mime, type FROM mime_type WHERE id=$1", m.QID(q)).Scan(&m.Name, &m.Type)
	if err != nil {
		log.Print(err)
	}

	return m.Type
}

func (m *Mime) Save(q querier) error {
	if m.QID(q) != 0 {
		return nil
	}

	if m.Name == "" || m.Type == "" {
		return errors.New("mime: not enough arguments")
	}

	err := q.QueryRow("INSERT INTO mime_type(mime, type) VALUES($1, $2) RETURNING id", m.Name, m.Type).Scan(&m.ID)
	if err != nil {
		return err
	}

	Mimes = append(Mimes, m)

	return nil
}

func (m *Mime) Parse(str string) error {
	splt := strings.Split(str, "/")

	if len(splt) != 2 {
		return errors.New("error splitting mime")
	}
	m.Name = splt[1]
	m.Type = splt[0]

	// if m.QID() != 0 {
	// 	return nil
	// }

	return nil
}

func (m Mime) String() string {
	return m.Str()
}

func (m Mime) Str() string {
	return m.QType(DB) + "/" + m.QName(DB)
}
