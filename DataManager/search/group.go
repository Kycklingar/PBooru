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

func (g *group) And(cond Condition) Where {
	g.Conditions = append(g.Conditions, AndCondition{cond})
	return g
}

func (g *group) Or(cond Condition) Where {
	g.Conditions = append(g.Conditions, OrCondition{cond})
	return g
}
