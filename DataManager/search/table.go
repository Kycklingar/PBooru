package search

import "fmt"

func NewTable(name, alias string) Table {
	return table{
		name:  name,
		alias: alias,
	}
}

type Table interface {
	Name() string
	Alias() string
	Field(field string) TableField
}

type TableField string

type table struct {
	name  string
	alias string
}

func (t table) Name() string {
	return t.name
}

func (t table) Alias() string {
	return t.alias
}

func (t table) Field(field string) TableField {
	return TableField(fmt.Sprint(t.alias, ".", field))
}

type IsNull TableField
type IsNotNull TableField

func (f IsNull) Condition(w Writer) {
	w.WriteString(string(f))
	w.WriteString(" IS NULL")
}

func (f IsNotNull) Condition(w Writer) {
	w.WriteString(string(f))
	w.WriteString(" IS NOT NULL")
}
