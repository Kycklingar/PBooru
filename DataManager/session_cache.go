package DataManager

import (
	"container/list"
	"sync"
	"time"
)

var sc *sessionCache

func init() {
	sc = &sessionCache{
		sessions: make(map[string]*list.Element),
		list:     list.New(),
	}
}

type sessionCache struct {
	sessions map[string]*list.Element
	list     *list.List

	s sync.Mutex
}

func (c *sessionCache) Push(s Session) {
	c.s.Lock()
	defer c.s.Unlock()

	el := c.list.PushFront(&s)
	c.sessions[s.Key] = el
}

func (c *sessionCache) Get(key string) *Session {
	c.s.Lock()
	defer c.s.Unlock()

	if e, ok := c.sessions[key]; ok {
		return e.Value.(*Session)
	}

	return nil
}

func (c *sessionCache) Update(key string) {
	c.s.Lock()
	defer c.s.Unlock()

	if e, ok := c.sessions[key]; ok {
		c.list.MoveToFront(e)

		s := e.Value.(*Session)
		s.Accessed = time.Now()
	}
}

func (c *sessionCache) Purge(key string) {
	c.s.Lock()
	defer c.s.Unlock()

	e, ok := c.sessions[key]
	if !ok {
		return
	}

	c.list.Remove(e)

	delete(c.sessions, key)
}

func (c *sessionCache) Last() *Session {
	e := c.list.Back()
	if e != nil {
		return e.Value.(*Session)
	}

	return nil
}
