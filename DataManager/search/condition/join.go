package condition

import "github.com/kycklingar/PBooru/DataManager/search"

func Join(left, right search.TableField) search.Condition {
	return join{
		left:  left,
		right: right,
	}
}

type join struct {
	left  search.TableField
	right search.TableField
}

func (jc join) Condition(w search.Writer) {
	w.WriteString(string(jc.left))
	w.WriteString(" = ")
	w.WriteString(string(jc.right))
}
