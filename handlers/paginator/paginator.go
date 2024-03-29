package paginate

import (
	"fmt"
	"math"
	"strconv"

	mm "github.com/kycklingar/MinMax"
)

func New(current, itemsTotal, itemsPP, pageButtons int, format string) Paginator {
	return Paginator{
		current: current,
		last:    int(math.Ceil(float64(itemsTotal) / float64(itemsPP))),
		plength: pageButtons,
		format:  format,
	}
}

type Paginator struct {
	// Current page
	current int
	// Last page
	last int
	// Max page buttons
	plength int
	// Linkformat
	format string
}

type page struct {
	Current bool
	Val     string
	Href    string
}

func (p Paginator) Paginate() []page {

	var pages []page

	if p.current > 1 {
		pages = append(pages, page{false, "First", fmt.Sprintf(p.format, 1)})
		pages = append(pages, page{false, "Previous", fmt.Sprintf(p.format, p.current-1)})
	}

	var start, end int
	var lendiv = p.plength / 2

	if p.current < lendiv { // Beginning
		start, end = 1, mm.Min(p.plength, p.last)
	} else if p.last-p.current < lendiv { // End
		start, end = mm.Max(1, p.last-p.plength+1), p.last
	} else { // Middle
		start, end = mm.Max(1, p.current-lendiv+1), p.current+lendiv
	}

	for current := start; current <= end; current++ {
		pages = append(pages, page{current == p.current, strconv.Itoa(current), fmt.Sprintf(p.format, current)})
	}

	if p.current < p.last {
		pages = append(pages, page{false, "Next", fmt.Sprintf(p.format, p.current+1)})
		pages = append(pages, page{false, "Last", fmt.Sprintf(p.format, p.last)})
	}

	return pages
}
