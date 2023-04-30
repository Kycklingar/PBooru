package postSearch

import "github.com/kycklingar/PBooru/DataManager/search"

type Posts struct {
	search.Table

	tables     []search.Eval
	conditions search.Conditions
}

func NewPostsSearch(alias string) Posts {
	return Posts{
		Table: search.NewTable("posts", alias),
	}
}

func (p *Posts) Join(table search.Table) search.On {
	var join = search.Join(table)
	p.tables = append(p.tables, &join)
	return &join
}

func (p *Posts) LeftJoin(table search.Table) search.On {
	var join = search.LeftJoin(table)
	p.tables = append(p.tables, &join)
	return &join
}

func (p *Posts) RightJoin(table search.Table) search.On {
	var join = search.RightJoin(table)
	p.tables = append(p.tables, &join)
	return &join
}

func (p *Posts) Where(condition search.Condition) search.Logic {
	if len(p.conditions) > 0 {
		return p.And(condition)
	}
	p.conditions = append(p.conditions, condition)
	return p
}

func (p *Posts) And(condition search.Condition) search.Logic {
	p.conditions = append(p.conditions, search.And(condition))
	return p
}

func (p *Posts) Or(condition search.Condition) search.Logic {
	p.conditions = append(p.conditions, search.Or(condition))
	return p
}

func (p *Posts) Eval(w search.Writer) {
	w.WriteString(p.Name())
	w.WriteRune(' ')
	w.WriteString(string(p.Alias()))

	for _, table := range p.tables {
		w.WriteRune('\n')
		table.Eval(w)
	}

	if len(p.conditions) > 0 {
		w.WriteString("\nWHERE ")
		p.conditions.Condition(w)
	}
}
