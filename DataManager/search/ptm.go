package search

import (
	"strconv"
)

type ptm struct {
	Table

	tables         []Search
	joinConditions Conditions

	tags []int
}

func NewPtm(name string, tag ...int) ptm {
	return ptm{
		Table: Table(name),
		tags:  tag,
	}
}

func (ptm *ptm) On(condition Condition) Join {
	ptm.joinConditions = append(ptm.joinConditions, condition)
	return ptm
}

func (ptm *ptm) Join(on On) On {
	ptm.tables = append(ptm.tables, InnerJoin{on})
	return on
}

func (ptm *ptm) LeftJoin(on On) On {
	ptm.tables = append(ptm.tables, LeftJoin{on})
	return on
}

func (ptm *ptm) RightJoin(on On) On {
	ptm.tables = append(ptm.tables, LeftJoin{on})
	return on
}

func (ptm *ptm) Eval(w Writer) {
	w.WriteString("post_tag_mappings ")
	w.WriteString(string(ptm.Table))
	w.WriteRune('\n')
	w.WriteString("ON ")

	ptm.joinConditions.Condition(w)

	for _, table := range ptm.tables {
		table.Eval(w)
		//w.WriteRune('\n')
	}
}

func (ptm *ptm) And(cond Condition) Join {
	ptm.joinConditions = append(ptm.joinConditions, AndCondition{cond})
	return ptm
}

func (ptm *ptm) Or(cond Condition) Join {
	ptm.joinConditions = append(ptm.joinConditions, OrCondition{cond})
	return ptm
}

func (ptm *ptm) Condition(w Writer) {
	w.WriteString(ptm.Field("tag_id"))

	switch len(ptm.tags) {
	default:
		ptm.write(w)
	case 1:
		ptm.writeOne(w)
	case 0:
	}
}

func (ptm *ptm) writeOne(w Writer) {
	w.WriteString(" = ")
	w.WriteString(strconv.Itoa(ptm.tags[0]))
}

func (ptm *ptm) write(w Writer) {
	w.WriteString(" IN (")

	w.WriteString(strconv.Itoa(ptm.tags[0]))
	for i := 1; i < len(ptm.tags); i++ {
		w.WriteRune(',')
		w.WriteString(strconv.Itoa(ptm.tags[i]))
	}

	w.WriteRune(')')
}
