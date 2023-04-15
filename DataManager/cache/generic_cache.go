package cache

import (
	"context"
	"sync"
	"time"

	list "github.com/bahlo/generic-list-go"
)

func NewGeneric[K comparable, T any](zero T, expireAfter time.Duration, maxLifeTime time.Duration) *Generic[K, T] {
	return &Generic[K, T]{
		zero:        zero,
		expireAfter: expireAfter,
		maxLifeTime: maxLifeTime,
		data:        make(map[K]*list.Element[tracer[K, T]]),
	}
}

type Generic[K comparable, T any] struct {
	sync.Mutex
	list.List[tracer[K, T]]

	// Used as return on missing elements
	zero T

	expireAfter time.Duration
	maxLifeTime time.Duration
	data        map[K]*list.Element[tracer[K, T]]
}

func newTracer[K comparable, T any](key K, item T) tracer[K, T] {
	return tracer[K, T]{
		key:     key,
		item:    item,
		access:  accessTime(time.Now()),
		created: time.Now(),
	}
}

// tracer tracks access time of an item
type tracer[K comparable, T any] struct {
	key     K
	item    T
	access  accessTime
	created time.Time
}

type accessTime time.Time

func (at *accessTime) touch() { *at = accessTime(time.Now()) }

func (c *Generic[K, T]) Get(key K) (T, bool) {
	c.Lock()
	defer c.Unlock()

	element, ok := c.data[key]
	if !ok {
		return c.zero, ok
	}

	element.Value.access.touch()
	c.MoveToFront(element)

	return element.Value.item, true
}

func (c *Generic[K, T]) Set(key K, value T) {
	c.Lock()
	defer c.Unlock()

	if element, ok := c.data[key]; ok {
		element.Value.item = value
		element.Value.access.touch()
		c.MoveToFront(element)
		return
	}

	c.data[key] = c.PushFront(newTracer(key, value))
}

func (c *Generic[K, T]) Del(key K) {
	c.Lock()
	defer c.Unlock()

	element, ok := c.data[key]
	if !ok {
		return
	}

	delete(c.data, key)
	c.Remove(element)
}

func (c *Generic[K, T]) GC(ctx context.Context) {
	var freq = c.expireAfter / 4
	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.purge()
		}

		time.Sleep(freq)
	}
}

func (c *Generic[K, T]) purge() {
	var expired = time.Now().Add(-c.expireAfter)

	c.Lock()
	defer c.Unlock()

	for e := c.Back(); e != nil; e = c.Back() {
		if time.Time(e.Value.access).After(expired) {
			break
		}

		delete(c.data, e.Value.key)
		c.Remove(e)
	}

	if c.maxLifeTime <= 0 {
		return
	}

	var maxLife = time.Now().Add(-c.maxLifeTime)
	for e := c.Front(); e != nil; e = e.Next() {
		if e.Value.created.After(maxLife) {
			continue
		}

		delete(c.data, e.Value.key)
		c.Remove(e)
	}
}
