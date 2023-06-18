package search

type Order interface {
	Eval
	By(field TableField, direction OrderDirection) Order
}

type OrderDirection string

const (
	Ascending  OrderDirection = "ASC"
	Descending OrderDirection = "DESC"
	Random     OrderDirection = "RANDOM()"
)
