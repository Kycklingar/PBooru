package postSearch

import (
	"github.com/kycklingar/PBooru/DataManager/search"
	"github.com/kycklingar/PBooru/DataManager/search/condition"
	"github.com/kycklingar/PBooru/DataManager/search/table"
)

type ptm struct {
	search.Table
	condition search.Condition
}

func NewPtm(alias string, tags ...int) ptm {
	var t = table.New("post_tag_mappings", alias)
	return ptm{
		Table:     t,
		condition: condition.List(t.Field("tag_id"), tags...),
	}
}

func (ptm ptm) Condition(w search.Writer) {
	ptm.condition.Condition(w)
}
