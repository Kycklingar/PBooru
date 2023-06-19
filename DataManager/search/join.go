package search

type On interface {
	Fields(TableField, TableField) On
	On(Condition) Logic
}
