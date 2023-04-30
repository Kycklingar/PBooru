package search

type Logic interface {
	And(Condition) Logic
	Or(Condition) Logic
}
