package condition

import "github.com/kycklingar/PBooru/DataManager/search"

func Equal(field search.TableField, value string) search.Condition {
	return equal{
		field: field,
		value: value,
	}
}

type equal struct {
	field search.TableField
	value string
}

func (e equal) Condition(w search.Writer) {
	w.WriteString(string(e.field))
	w.WriteString(" = ")
	w.WriteString(e.value)
}
