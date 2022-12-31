package session

import (
	"time"
)

type Session struct {
	Key   Key
	Value any

	access time.Time
}

func (s *Session) touch() {
	s.access = time.Now()
}
