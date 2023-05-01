package search

import "strconv"

type Condition interface {
	Condition(b Writer)
}

// And condition wrapper
func And(condition Condition) Condition {
	return andCondition{c: condition}
}

// Or condition wrapper
func Or(condition Condition) Condition {
	return orCondition{c: condition}
}

func JoinCondition(left, right TableField) Condition {
	return joinCondition{
		left:  left,
		right: right,
	}
}

func Boolean(field TableField, boolean bool) Condition {
	return boolCondition{
		field:   field,
		boolean: boolean,
	}
}

type andCondition struct {
	c Condition
}

func (ac andCondition) Condition(w Writer) {
	w.WriteString("AND ")
	ac.c.Condition(w)
}

type orCondition struct {
	c Condition
}

func (oc orCondition) Condition(w Writer) {
	w.WriteString("OR ")
	oc.c.Condition(w)
}

type joinCondition struct {
	left  TableField
	right TableField
}

func (jc joinCondition) Condition(w Writer) {
	w.WriteString(string(jc.left))
	w.WriteString(" = ")
	w.WriteString(string(jc.right))
}

type boolCondition struct {
	field   TableField
	boolean bool
}

func (b boolCondition) Condition(w Writer) {
	w.WriteString(string(b.field))
	w.WriteString(" = ")
	w.WriteString(strconv.FormatBool(b.boolean))
}
