package user

import (
	"math"

	mm "github.com/kycklingar/MinMax"
)

var titles = [...]string{
	"Lurker",
	"Contributor",
	"Tagger",
	"Respectable",
	"Godlike",
	"Archivist",
	"Archdaemon",
}

func title(logCount int) string {
	if logCount <= 0 {
		return ""
	}

	return titles[mm.Min(
		int(math.Log10(float64(logCount))),
		len(titles)-1,
	)]
}
