package search

type Joiner interface {
	Fields(TableField, TableField) Logic
	On(Condition) Logic
}
