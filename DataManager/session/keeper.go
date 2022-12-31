package session

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type element = list.Element

func NewKeeper(store Store, offloadAfter time.Duration) *Keeper {
	var keeper = Keeper{
		session:      make(map[Key]*element),
		Store:        store,
		offloadAfter: offloadAfter,
	}
	keeper.Init()

	return &keeper
}

type Keeper struct {
	sync.Mutex
	list.List
	Store

	offloadAfter time.Duration
	session      map[Key]*element
}

func (k *Keeper) Get(key Key) (any, error) {
	k.Lock()
	defer k.Unlock()

	if e, ok := k.session[key]; ok {
		session := e.Value.(Session)
		session.touch()
		e.Value = session
		return session.Value, nil
	}

	value, err := k.Retrieve(key)
	if err != nil {
		return nil, err
	}

	k.session[key] = k.PushFront(Session{
		Key:    key,
		Value:  value,
		access: time.Now(),
	})

	return value, nil
}

func (k *Keeper) Keep(value any) (Key, error) {
	session := Session{
		Key:    generateKey(64),
		Value:  value,
		access: time.Now(),
	}

	k.Lock()
	defer k.Unlock()

	k.session[session.Key] = k.PushFront(session)

	return session.Key, k.Put(session)
}

func (k *Keeper) Destroy(key Key) error {
	k.Lock()
	defer k.Unlock()

	s, ok := k.session[key]
	if ok {
		delete(k.session, key)
		k.Remove(s)
	}

	return k.Erase(key)
}

func (k *Keeper) GC(ctx context.Context) {
	var next time.Time
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if next.Before(time.Now()) {
			next = time.Now().Add(k.offloadAfter / 3)
			k.gc()
		}

		time.Sleep(time.Second)
	}
}

func (k *Keeper) gc() {
	k.Lock()
	defer k.Unlock()

	var expunged []Session

	var offload = time.Now().Add(-k.offloadAfter)
	for e := k.Back(); e != nil; e = k.Back() {
		// the rest of sessions were touched past offload time
		if e.Value.(Session).access.After(offload) {
			break
		}

		// Remove the session from keeper and store it in the store
		session := k.Remove(e).(Session)
		delete(k.session, session.Key)
		expunged = append(expunged, session)
	}

	k.UpdateExpiration(expunged...)

	k.CollectExpired()
}
