package condition

import (
	"time"

	"github.com/kycklingar/PBooru/DataManager/search"
)

type operator string

const (
	format = "2006-01-02 15:04:05"

	GT operator = ">"
)

func Time(field search.TableField, op operator, time time.Time) search.Condition {
	return date{
		field:    field,
		operator: op,
		time:     time,
	}
}

type date struct {
	field    search.TableField
	operator operator
	time     time.Time
}

func (d date) Condition(w search.Writer) {
	w.WriteString(string(d.field))
	w.WriteRune(' ')
	w.WriteString(string(d.operator))
	w.WriteRune(' ')
	w.WriteRune('\'')
	w.WriteString(d.time.Format(format))
	w.WriteRune('\'')
}
