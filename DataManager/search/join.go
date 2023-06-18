package search

type On interface {
	On(Condition) Logic
}
