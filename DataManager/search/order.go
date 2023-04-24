package search

type order string

const (
	Ascending  order = "ASC"
	Descending order = "DESC"
	Random     order = "RANDOM()"
)
