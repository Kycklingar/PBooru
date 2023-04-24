package search

type Where interface {
	And(Condition) Where
	Or(Condition) Where
}
