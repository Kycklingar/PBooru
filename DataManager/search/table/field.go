package table

import "github.com/kycklingar/PBooru/DataManager/search"

type (
	IsNull    search.TableField
	IsNotNull search.TableField
)

func (f IsNull) Condition(w search.Writer) {
	w.WriteString(string(f))
	w.WriteString(" IS NULL")
}

func (f IsNotNull) Condition(w search.Writer) {
	w.WriteString(string(f))
	w.WriteString(" IS NOT NULL")
}
