package condition

import "github.com/kycklingar/PBooru/DataManager/search"

func Group(cond search.Condition) *group {
	return &group{search.Conditions{cond}}
}

type group struct {
	search.Conditions
}

func (g group) Condition(w search.Writer) {
	w.WriteString("(\n")
	g.Conditions.Prefixed(w, "\t")
	w.WriteString(")")
}

func (g *group) And(cond search.Condition) search.Logic {
	g.Conditions = append(g.Conditions, search.And(cond))
	return g
}

func (g *group) Or(cond search.Condition) search.Logic {
	g.Conditions = append(g.Conditions, search.Or(cond))
	return g
}
