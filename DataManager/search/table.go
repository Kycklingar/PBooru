package search

type Table interface {
	Name() string
	Alias() string
	Field(field string) TableField
}

type TableField string
