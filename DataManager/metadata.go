package DataManager

import (
	"fmt"
	"strings"
	"time"

	"github.com/kycklingar/PBooru/DataManager/namespace"
)

type MetaData interface {
	String() string

	Namespace() namespace.Namespace
	Data() string
	value() interface{}
}

func parseMetaDataString(metaStr string) ([]MetaData, error) {
	var retu []MetaData

	for _, m := range strings.Split(metaStr, "\n") {
		md, err := parseMetaData(m)
		if err != nil {
			return nil, err
		}

		if md != nil {
			retu = append(retu, md)
		}
	}

	return retu, nil
}

func parsePartialDate(format, date string) (time.Time, error) {
	var l = len(date)
	if l > len(format) || l < 4 {
		return time.Time{}, fmt.Errorf("invalid date: %s, expected form is 2022-07-28", date)
	}

	return time.Parse(format[:l], date)
}

func parseMetaData(mstr string) (MetaData, error) {
	data := strings.SplitN(strings.TrimSpace(mstr), ":", 2)
	if len(data) < 2 {
		return nil, nil
	}

	nspace, value := data[0], data[1]

	if nspace == "date" {
		t, err := parsePartialDate(iso8601, value)
		if err != nil {
			return nil, err
		}

		return metaDate(t), nil
	}

	var (
		md  = metadata{data: value}
		err error
	)
	md.namespace, _, err = namespace.New(DB, nspace)

	return md, err
}

type metadata struct {
	namespace namespace.Namespace
	data      string
}

func (m metadata) Namespace() namespace.Namespace { return m.namespace }
func (m metadata) Data() string                   { return m.data }
func (m metadata) String() string                 { return string(m.namespace) + ":" + m.data }
func (m metadata) value() interface{}             { return m.data }

const iso8601 = "2006-01-02"

type metaDate time.Time

func (m metaDate) Namespace() namespace.Namespace { return namespace.Namespace("date") }
func (m metaDate) Data() string                   { return time.Time(m).Format(iso8601) }
func (m metaDate) String() string                 { return "date" + ":" + m.Data() }
func (m metaDate) value() interface{}             { return time.Time(m) }
