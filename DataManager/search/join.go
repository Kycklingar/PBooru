package search

type Join interface {
	Search
	And(Condition) Join
	Or(Condition) Join
}

type InnerJoin struct {
	Search
}

func (j InnerJoin) Eval(w Writer) {
	w.WriteString("JOIN ")
	j.Search.Eval(w)
}

type LeftJoin struct {
	Search
}

func (j LeftJoin) Eval(w Writer) {
	w.WriteString("LEFT JOIN ")
	j.Search.Eval(w)
}

type RightJoin struct {
	Search
}

func (j RightJoin) Eval(w Writer) {
	w.WriteString("RIGHT JOIN ")
	j.Search.Eval(w)
}
