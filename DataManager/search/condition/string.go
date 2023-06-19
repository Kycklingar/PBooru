package condition

import "github.com/kycklingar/PBooru/DataManager/search"

type String string

func (s String) Condition(w search.Writer) {
	w.WriteString(string(s))
}
