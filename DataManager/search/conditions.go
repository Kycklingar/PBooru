package search

type Conditions []Condition

func (c Conditions) Condition(w Writer) {
	for _, cond := range c {
		cond.Condition(w)
		w.WriteRune('\n')
	}
}

func (c Conditions) Prefixed(w Writer, prefix string) {
	for _, cond := range c {
		w.WriteString(prefix)
		cond.Condition(w)
		w.WriteRune('\n')
	}
}
