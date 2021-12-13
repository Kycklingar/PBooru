package DataManager

import (
	"log"
	"strings"
	"time"
)

type MetaData interface {
	String() string

	Namespace() string
	Data() string
	value() interface{}
}

func parseMetaDataString(metaStr string) []MetaData {
	var retu []MetaData

	for _, m := range strings.Split(metaStr, "\n") {
		if md := parseMetaData(m); md != nil {
			retu = append(retu, md)
		}
	}

	return retu
}

func parseMetaData(mstr string) MetaData {
	data := strings.SplitN(strings.TrimSpace(mstr), ":", 2)
	if len(data) < 2 {
		return nil
	}

	namespace, value := data[0], data[1]

	if namespace == "date" {
		t, err := time.Parse(iso8601, value)
		if err != nil {
			log.Println(err)
			return nil
		}

		return metaDate(t)
	}

	return metadata{
		namespace: data[0],
		data:      data[1],
	}
}

type metadata struct {
	namespace string
	data      string
}

func (m metadata) Namespace() string  { return m.namespace }
func (m metadata) Data() string       { return m.data }
func (m metadata) String() string     { return m.namespace + ":" + m.data }
func (m metadata) value() interface{} { return m.data }

const iso8601 = "2006-01-02"

type metaDate time.Time

func (m metaDate) Namespace() string  { return "date" }
func (m metaDate) Data() string       { return time.Time(m).Format(iso8601) }
func (m metaDate) String() string     { return "date" + ":" + m.Data() }
func (m metaDate) value() interface{} { return m }
