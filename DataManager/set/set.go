package set

import (
	"fmt"
	"strings"
)

type nothing struct{}

var empty = nothing{}

type Set[T comparable] map[T]nothing

func (s Set[T]) Add(n ...T) {
	for _, v := range n {
		s[v] = empty
	}
}

func (s Set[T]) Has(n T) bool {
	_, ok := s[n]
	return ok
}

func (s Set[T]) Remove(n ...T) {
	for _, v := range n {
		delete(s, v)
	}
}

// String returns a comma separated string of the values
func (s Set[T]) String() string {
	var res = make([]string, 0, len(s))

	for k, _ := range s {
		res = append(res, fmt.Sprint(k))
	}

	return strings.Join(res, ",")
}

func (s Set[T]) Slice() []T {
	var slice = make([]T, 0, len(s))

	for k, _ := range s {
		slice = append(slice, k)
	}

	return slice
}
