package search

type Table interface {
	Eval
	Name() string
	Alias() string
	Field(field string) TableField
}

type TableField string
