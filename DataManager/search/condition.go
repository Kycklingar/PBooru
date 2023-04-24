package search

type Condition interface {
	Condition(b Writer)
}

// And condition wrapper
type AndCondition struct {
	c Condition
}

func (ac AndCondition) Condition(w Writer) {
	w.WriteString("AND ")
	ac.c.Condition(w)
}

// Or condition wrapper
type OrCondition struct {
	c Condition
}

func (oc OrCondition) Condition(w Writer) {
	w.WriteString("OR ")
	oc.c.Condition(w)
}

type JoinCondition struct {
	left  string
	right string
}

func (jc JoinCondition) Condition(w Writer) {
	w.WriteString(jc.left)
	w.WriteString(" = ")
	w.WriteString(jc.right)
}
