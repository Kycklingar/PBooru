package search

type Search interface {
	Join(On) On
	LeftJoin(On) On
	RightJoin(On) On
	Eval(b Writer)
}

type On interface {
	Search
	On(Condition) Join
}
