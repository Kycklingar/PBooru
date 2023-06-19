package order

import "github.com/kycklingar/PBooru/DataManager/search"

const (
	Ascending  search.OrderDirection = "ASC"
	Descending search.OrderDirection = "DESC"
	Random     search.OrderDirection = "RANDOM()"
)

func By(field search.TableField, direction search.OrderDirection) search.Order {
	return new(ordering).By(field, direction)
}

type ordering []order

func (o *ordering) By(field search.TableField, direction search.OrderDirection) search.Order {
	*o = append(*o, order{
		field:     field,
		direction: direction,
	})
	return o
}

func (o ordering) Eval(w search.Writer) {
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
	direction search.OrderDirection
	field     search.TableField
}

func (o order) Eval(w search.Writer) {
	switch o.direction {
	case Random:
		w.WriteString(string(Random))
	default:
		w.WriteString(string(o.field))
		w.WriteRune(' ')
		w.WriteString(string(o.direction))
	}
}
