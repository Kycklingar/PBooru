package condition

import (
	"strconv"

	"github.com/kycklingar/PBooru/DataManager/search"
)

func List(field search.TableField, values ...int) search.Condition {
	return list{
		field:  field,
		values: values,
	}
}

type list struct {
	field  search.TableField
	values []int
}

func (l list) Condition(w search.Writer) {
	if len(l.values) <= 0 {
		return
	}

	w.WriteString(string(l.field))
	w.WriteString(" IN (")
	w.WriteString(strconv.Itoa(l.values[0]))

	for i := 1; i < len(l.values); i++ {
		w.WriteRune(',')
		w.WriteString(strconv.Itoa(l.values[i]))
	}

	w.WriteRune(')')
}
