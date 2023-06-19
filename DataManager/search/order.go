package search

type Order interface {
	Eval
	By(field TableField, direction OrderDirection) Order
}

type OrderDirection string
