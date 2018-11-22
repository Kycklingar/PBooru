package cache

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
	"time"
)

var Cache *cacher

type Data interface {
	Set(...interface{}) error
	Get() Data
}

type Identifier interface {
	toString() string
}

type cache struct {
	lastAccess time.Time
	id         string
	kind       string
	data       interface{}
}

type cacher struct {
	mapped  map[string]*list.Element
	maplist *list.List

	l sync.Mutex
}

func (c *cacher) Get(kind, id string) interface{} {
	//fmt.Println("Accessing ", kind, id)
	c.l.Lock()
	defer c.l.Unlock()
	if t, ok := c.mapped[kind+" "+id]; ok {
		//fmt.Println("GET", kind, id)
		c.update(t)
		return t.Value.(*cache).data
	}
	return nil
}

func (c *cacher) update(el *list.Element) {
	el.Value.(*cache).lastAccess = time.Now()
	//fmt.Println("Updating", el.Value.(*cache).kind, el.Value.(*cache).id, el.Value.(*cache).lastAccess)
	c.maplist.MoveToFront(el)
}

func (c *cacher) Set(kind, id string, data interface{}) {
	c.l.Lock()
	defer c.l.Unlock()
	//fmt.Println("SET", kind, id)
	if v, ok := c.mapped[kind+" "+id]; ok {
		v.Value.(*cache).data = data
		c.update(v)
	} else {
		var cach = &cache{}
		cach.data = data
		cach.kind = kind
		cach.id = id
		cach.lastAccess = time.Now()
		c.mapped[kind+" "+id] = c.maplist.PushFront(cach)
	}
}

func (c *cacher) Purge(kind string, id string) {
	c.l.Lock()
	defer c.l.Unlock()
	//fmt.Println("PURGE", kind, id)
	for _, m := range c.mapped {
		ca := m.Value.(*cache)
		if ca.kind == kind {
			for _, i := range strings.Split(ca.id, " ") {
				//fmt.Println(ca.kind, ca.id)
				if i == id {
					//fmt.Println("Cache: removing ", ca.kind, ca.id)
					delete(c.mapped, ca.kind+" "+ca.id)
					c.maplist.Remove(m)
				}
			}
		}
	}
}

var maxLifetTime = time.Hour / 3

func (c *cacher) GC() {
	//fmt.Println("Running GC")
	c.l.Lock()
	defer c.l.Unlock()
	bef := len(c.mapped)
	for {
		e := c.maplist.Back()
		if e == nil {
			break
		}
		//fmt.Println("GC", e.Value.(*cache).lastAccess, e.Value.(*cache).kind, e.Value.(*cache).id)
		if (e.Value.(*cache).lastAccess.Add(maxLifetTime).Unix()) < time.Now().Unix() {
			// if e.Value.(*cache).kind == "PC" {
			// 	fmt.Println(fmt.Sprintf("Running GC on %s, t %s", e.Value.(*cache).id, e.Value.(*cache).lastAccess))
			// }
			c.maplist.Remove(e)
			delete(c.mapped, e.Value.(*cache).kind+" "+e.Value.(*cache).id)
		} else {
			break
		}
	}
	fmt.Println("GC'd ", bef-len(c.mapped), "items.", len(c.mapped), "items left")
	//fmt.Println("GC Done")
	time.AfterFunc(maxLifetTime, c.GC)
}

func init() {
	Cache = &cacher{mapped: make(map[string]*list.Element), maplist: list.New()}
	time.AfterFunc(maxLifetTime, Cache.GC)
}
