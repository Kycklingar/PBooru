package postSearch

import (
	"strconv"

	"github.com/kycklingar/PBooru/DataManager/search"
)

func Mimes(field search.TableField, mimeIDs []int) mimes {
	return mimes{
		field: field,
		mimes: mimeIDs,
	}
}

type mimes struct {
	field search.TableField
	mimes []int
}

func (m mimes) Condition(w search.Writer) {
	w.WriteString(string(m.field))
	w.WriteString(" IN (")
	w.WriteString(strconv.Itoa(m.mimes[0]))

	for i := 1; i < len(m.mimes); i++ {
		w.WriteRune(',')
		w.WriteString(strconv.Itoa(m.mimes[i]))
	}

	w.WriteRune(')')
}
