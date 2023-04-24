package search

import "fmt"

type Table string

func (t Table) Field(name string) string {
	return fmt.Sprint(t, ".", name)
}

type IsNull string
type IsNotNull string

func (f IsNull) Condition(w Writer) {
	w.WriteString(string(f))
	w.WriteString(" IS NULL")
}

func (f IsNotNull) Condition(w Writer) {
	w.WriteString(string(f))
	w.WriteString(" IS NOT NULL")
}
