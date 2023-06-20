package table

import (
	"github.com/kycklingar/PBooru/DataManager/search"
	"github.com/kycklingar/PBooru/DataManager/search/condition"
)

type joinType string

const (
	innerJoin joinType = "JOIN"
	leftJoin  joinType = "LEFT JOIN"
	rightJoin joinType = "RIGHT JOIN"
)

type join struct {
	jointype   joinType
	table      search.Table
	conditions search.Conditions
}

func (j join) Eval(w search.Writer) {
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

func (j *join) On(condition search.Condition) search.Logic {
	j.conditions = append(j.conditions, condition)
	return j
}

func (j *join) Fields(left, right search.TableField) search.Logic {
	return j.On(condition.Join(left, right))
}

func (j *join) And(condition search.Condition) search.Logic {
	return j.On(search.And(condition))
}

func (j *join) Or(condition search.Condition) search.Logic {
	return j.On(search.Or(condition))
}
