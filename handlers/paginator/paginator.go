package paginate

import (
	"fmt"
	"strconv"
)

type Paginator struct {
	Current int
	Last    int
	Plength int

	Format string
}

type page struct {
	Current bool
	Val     string
	Href    string
}

func (p Paginator) Paginate() []page {

	var pages []page

	if p.Current > 1 {
		pages = append(pages, page{false, "First", fmt.Sprintf(p.Format, 1)})
		pages = append(pages, page{false, "Previous", fmt.Sprintf(p.Format, p.Current-1)})
	}

	var start, end int
	var lendiv = p.Plength / 2

	if p.Current < lendiv { // Beginning
		start, end = 1, Max(p.Plength, p.Last)
	} else if p.Last-p.Current < lendiv { // End
		start, end = Min(1, p.Last-p.Plength+1), p.Last
	} else { // Middle
		start, end = Min(1, p.Current-lendiv+1), p.Current+lendiv
	}

	for current := start; current <= end; current++ {
		pages = append(pages, page{current == p.Current, strconv.Itoa(current), fmt.Sprintf(p.Format, current)})
	}

	if p.Current < p.Last {
		pages = append(pages, page{false, "Next", fmt.Sprintf(p.Format, p.Current+1)})
		pages = append(pages, page{false, "Last", fmt.Sprintf(p.Format, p.Last)})
	}

	return pages
}

func Min(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a < b {
		return a
	}
	return b
}
