package search

type Joiner interface {
	Fields(TableField, TableField) Joiner
	On(Condition) Logic
}
