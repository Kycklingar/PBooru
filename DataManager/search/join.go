package search

func Join(table Table) join {
	return newJoin(innerJoin, table)
}

func LeftJoin(table Table) join {
	return newJoin(leftJoin, table)
}

func RightJoin(table Table) join {
	return newJoin(rightJoin, table)
}

type Joiner interface {
	Join(Table) On
	LeftJoin(Table) On
	RightJoin(Table) On
}

type On interface {
	On(Condition) Logic
}

type joinType string

const (
	innerJoin joinType = "JOIN"
	leftJoin  joinType = "LEFT JOIN"
	rightJoin joinType = "RIGHT JOIN"
)

type join struct {
	jointype   joinType
	table      Table
	conditions Conditions
}

func newJoin(joinType joinType, table Table) join {
	return join{
		jointype: joinType,
		table:    table,
	}
}

func (j join) Eval(w Writer) {
	w.WriteString(string(j.jointype))
	w.WriteRune(' ')
	w.WriteString(j.table.Name())
	w.WriteRune(' ')
	w.WriteString(j.table.Alias())

	if len(j.conditions) > 0 {
		w.WriteRune('\n')
		w.WriteString("ON ")
	}

	for _, cond := range j.conditions {
		cond.Condition(w)
		w.WriteRune('\n')
	}
}

func (j *join) On(condition Condition) Logic {
	j.conditions = append(j.conditions, condition)
	return j
}

func (j *join) And(condition Condition) Logic {
	return j.On(And(condition))
}

func (j *join) Or(condition Condition) Logic {
	return j.On(Or(condition))
}
