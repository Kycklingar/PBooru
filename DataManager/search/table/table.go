package table

import (
	"fmt"

	"github.com/kycklingar/PBooru/DataManager/search"
)

func New(name, alias string) Table {
	return Table{
		name:  name,
		alias: alias,
	}
}

type Table struct {
	name  string
	alias string

	joins      []search.Eval
	conditions search.Conditions
}

func (t Table) Name() string {
	return t.name
}

func (t Table) Alias() string {
	return t.alias
}

func (t Table) Field(field string) search.TableField {
	return search.TableField(fmt.Sprint(t.alias, ".", field))
}

func (t *Table) Join(table search.Table) search.On {
	return t.addJoin(innerJoin, table)
}

func (t *Table) LeftJoin(table search.Table) search.On {
	return t.addJoin(leftJoin, table)
}

func (t *Table) RightJoin(table search.Table) search.On {
	return t.addJoin(rightJoin, table)
}

func (t *Table) addJoin(jointype joinType, table search.Table) search.On {
	var join = join{
		jointype: jointype,
		table:    table,
	}
	t.joins = append(t.joins, &join)
	return &join
}

func (t *Table) Where(condition search.Condition) search.Logic {
	if len(t.conditions) > 0 {
		return t.And(condition)
	}

	t.conditions.Add(condition)
	return t
}

func (t *Table) And(condition search.Condition) search.Logic {
	t.conditions.Add(search.And(condition))
	return t
}

func (t *Table) Or(condition search.Condition) search.Logic {
	t.conditions.Add(search.Or(condition))
	return t
}

func (t Table) Eval(w search.Writer) {
	w.WriteString(t.name)
	w.WriteRune(' ')
	w.WriteString(t.alias)

	for _, join := range t.joins {
		w.WriteRune('\n')
		join.Eval(w)
	}

	if len(t.conditions) > 0 {
		w.WriteString("\nWHERE ")
		t.conditions.Condition(w)
	}
}
