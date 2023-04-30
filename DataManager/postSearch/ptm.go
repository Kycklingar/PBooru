package postSearch

import (
	"strconv"

	"github.com/kycklingar/PBooru/DataManager/search"
)

type ptm struct {
	search.Table

	tables         []search.Table
	joinConditions search.Conditions

	tags []int
}

func NewPtm(alias string, tag ...int) ptm {
	return ptm{
		Table: search.NewTable("post_tag_mappings", alias),
		tags:  tag,
	}
}

//func (ptm *ptm) On(condition search.Condition) search.Joiner {
//	ptm.joinConditions = append(ptm.joinConditions, condition)
//	return ptm
//}
//
//func (ptm *ptm) Join(on search.On) search.On {
//	ptm.tables = append(ptm.tables, search.Join(on))
//	return on
//}
//
//func (ptm *ptm) LeftJoin(on search.On) search.On {
//	ptm.tables = append(ptm.tables, search.LeftJoin(on))
//	return on
//}
//
//func (ptm *ptm) RightJoin(on search.On) search.On {
//	ptm.tables = append(ptm.tables, search.RightJoin(on))
//	return on
//}
//
//func (ptm *ptm) Eval(w search.Writer) {
//	w.WriteString("post_tag_mappings ")
//	w.WriteString(string(ptm.TableAlias))
//	w.WriteRune('\n')
//	w.WriteString("ON ")
//
//	ptm.joinConditions.Condition(w)
//
//	for _, table := range ptm.tables {
//		//w.WriteRune('\n') ???
//		table.Eval(w)
//	}
//}

//func (ptm *ptm) And(cond search.Condition) search.Joiner {
//	ptm.joinConditions = append(ptm.joinConditions, search.And(cond))
//	return ptm
//}
//
//func (ptm *ptm) Or(cond search.Condition) search.Joiner {
//	ptm.joinConditions = append(ptm.joinConditions, search.Or(cond))
//	return ptm
//}

func (ptm *ptm) Condition(w search.Writer) {
	w.WriteString(string(ptm.Field("tag_id")))

	switch len(ptm.tags) {
	default:
		ptm.write(w)
	case 1:
		ptm.writeOne(w)
	case 0:
	}
}

func (ptm *ptm) writeOne(w search.Writer) {
	w.WriteString(" = ")
	w.WriteString(strconv.Itoa(ptm.tags[0]))
}

func (ptm *ptm) write(w search.Writer) {
	w.WriteString(" IN (")

	w.WriteString(strconv.Itoa(ptm.tags[0]))
	for i := 1; i < len(ptm.tags); i++ {
		w.WriteRune(',')
		w.WriteString(strconv.Itoa(ptm.tags[i]))
	}

	w.WriteRune(')')
}
