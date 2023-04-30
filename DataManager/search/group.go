package search

func Group(cond Condition) *group {
	return &group{Conditions{cond}}
}

type group struct {
	Conditions
}

func (g group) Condition(w Writer) {
	w.WriteString("(\n")
	g.Conditions.Prefixed(w, "\t")
	w.WriteString(")")
}

func (g *group) And(cond Condition) Logic {
	g.Conditions = append(g.Conditions, andCondition{cond})
	return g
}

func (g *group) Or(cond Condition) Logic {
	g.Conditions = append(g.Conditions, orCondition{cond})
	return g
}
