package search

type Order interface {
	Eval
	By(field TableField, direction orderDirection) Order
}

func NewOrder() Order {
	return new(ordering)
}

type orderDirection string

const (
	Ascending  orderDirection = "ASC"
	Descending orderDirection = "DESC"
	Random     orderDirection = "RANDOM()"
)

type ordering []order

func (o *ordering) By(field TableField, direction orderDirection) Order {
	*o = append(*o, order{
		field:     field,
		direction: direction,
	})
	return o
}

func (o ordering) Eval(w Writer) {
	if len(o) <= 0 {
		return
	}

	w.WriteString("\nORDER BY ")
	o[0].Eval(w)

	for i := 1; i < len(o); i++ {
		w.WriteString(", ")
		o[i].Eval(w)
	}
}

type order struct {
	direction orderDirection
	field     TableField
}

func (o order) Eval(w Writer) {
	switch o.direction {
	case Random:
		w.WriteString(string(Random))
	default:
		w.WriteString(string(o.field))
		w.WriteRune(' ')
		w.WriteString(string(o.direction))
	}
}
