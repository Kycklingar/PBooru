package search

import "fmt"

type Parameters []any

func (p *Parameters) New(param any) string {
	*p = append(*p, param)
	return fmt.Sprint("$", len(*p))
}
