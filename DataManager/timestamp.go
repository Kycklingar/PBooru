package DataManager

import (
	"fmt"
	"time"
)

const (
	displayTimestamp  = "2006-01-02 15:04:05"
	postgresTimestamp = "2006-01-02T15:04:05.000000Z"
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
