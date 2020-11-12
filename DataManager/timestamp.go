package DataManager

import (
	"fmt"
	"time"
)

const (
	displayTimestamp  = "2006-01-02 15:04:05"
	postgresTimestamp = "2006-01-02T15:04:05.000000Z"
)

const (
	day = time.Hour * 24
	week = day * 7
	month = week * 4
	year = month * 12
)

type timestamp struct {
	time *time.Time
}

func (t timestamp) Time() *time.Time {
	return t.time
}

func (t timestamp) String() string {
	if t.time != nil {
		return t.time.Format(displayTimestamp)
	}
	return ""
}

func (t *timestamp) Scan(data interface{}) error {
	var err error
	switch v := data.(type) {
	case nil:
	case time.Time:
		t.time = &v
	default:
		err = fmt.Errorf("timestamp incorrect type", data)
	}

	return err
}

func (t *timestamp) Elapsed() string {
	var elapsed string

	e := time.Since(*t.time)

	type u struct {
		d time.Duration
		s string
	}

	str := func(un u, e time.Duration) string {
		if e - un.d  < un.d {
			return un.s
		}
		return un.s + "s"
	}

	var units []u = []u{
		u{year, "Year"},
		u{month, "Month"},
		u{week, "Week"},
		u{day, "Day"},
		u{time.Hour, "Hour"},
		u{time.Minute, "Minute"},
		u{time.Second, "Second"},
		u{time.Millisecond, "Milliseconds"},
	}

	var unit int
	for i := 0; i < len(units); i++ {
		if e > units[i].d {
			unit = i
			break
		}
	}

	elapsed = fmt.Sprintf(
		"%d %s",
		e/units[unit].d,
		str(units[unit], e),
	)

	unit++
	if unit < len(units) {
		mod := e%units[unit-1].d/units[unit].d
		if mod > 0 {
			elapsed += fmt.Sprintf(
				" %d %s",
				mod,
				str(
					units[unit],
					mod*units[unit].d,
				),
			)
		}
	}


	return elapsed + " ago"
}
