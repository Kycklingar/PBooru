package condition

import (
	"strconv"

	"github.com/kycklingar/PBooru/DataManager/search"
)

func Boolean(field search.TableField, boolean bool) search.Condition {
	return boolCondition{
		field:   field,
		boolean: boolean,
	}
}

type boolCondition struct {
	field   search.TableField
	boolean bool
}

func (b boolCondition) Condition(w search.Writer) {
	w.WriteString(string(b.field))
	w.WriteString(" = ")
	w.WriteString(strconv.FormatBool(b.boolean))
}
