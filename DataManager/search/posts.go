package search

type posts struct {
	Table

	tables     []Search
	conditions Conditions
}

func NewPosts(name string) posts {
	return posts{
		Table: Table(name),
	}
}

func (p *posts) Join(table On) On {
	p.tables = append(p.tables, InnerJoin{table})
	return table
}

func (p *posts) LeftJoin(table On) On {
	p.tables = append(p.tables, LeftJoin{table})
	return table
}

func (p *posts) RightJoin(table On) On {
	p.tables = append(p.tables, RightJoin{table})
	return table
}

func (p *posts) Where(condition Condition) Where {
	p.conditions = append(p.conditions, condition)
	return p
}

func (p *posts) And(condition Condition) Where {
	p.conditions = append(p.conditions, AndCondition{condition})
	return p
}

func (p *posts) Or(condition Condition) Where {
	p.conditions = append(p.conditions, OrCondition{condition})
	return p
}

func (p *posts) Eval(w Writer) {
	w.WriteString("SELECT *\n")
	w.WriteString("FROM posts ")
	w.WriteString(string(p.Table))
	w.WriteRune('\n')
	for _, table := range p.tables {
		table.Eval(w)
		w.WriteRune('\n')
	}

	w.WriteString("WHERE ")
	p.conditions.Condition(w)
}
