package sqlbinder

type Binder interface {
	BindField(*Selection, Field)
}

type Field int

type fieldAddress struct {
	addr interface{}
	name string
	join string
}

type Selection struct {
	vals []fieldAddress
}

func (sel *Selection) Bind(addr interface{}, name, join string) {
	sel.vals = append(sel.vals, fieldAddress{addr, name, join})
}

func (sel *Selection) ReBind(addr... interface{}) {
	for i, _ := range addr {
		sel.vals[i].addr = addr[i]
	}
}

func (sel *Selection) Len() int {
	return len(sel.vals)
}

func (sel Selection) Values() []interface{} {
	var vals = make([]interface{}, len(sel.vals))
	for i, _ := range sel.vals {
		vals[i] = sel.vals[i].addr
	}

	return vals
}

func (sel Selection) Joins() (join string) {
	for _, v := range sel.vals {
		if v.join != "" {
			join += v.join + "\n"
		}
	}

	return
}

func (sel Selection) Select() (ret string) {
	for i, v := range sel.vals {
		ret += v.name
		if i < len(sel.vals) - 1 {
			ret += ", "
		}
	}

	ret += " "

	return
}

func BindFieldAddresses(b Binder, fields... Field) Selection {
	var sel Selection

	for _, f := range fields {
		b.BindField(&sel, f)
	}

	return sel
}
