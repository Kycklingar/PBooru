package search

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
