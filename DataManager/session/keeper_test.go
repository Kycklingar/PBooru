package session

import (
	"testing"
	"time"
)

func newMockStore(expire time.Duration) mockStore {
	return mockStore{
		session: make(map[Key]Session),
		expire:  expire,
	}
}

type mockStore struct {
	session map[Key]Session
	expire  time.Duration
}

func (mock mockStore) Retrieve(key Key) (any, error) {
	if v, ok := mock.session[key]; ok {
		return v.Value, nil
	}

	return nil, NotFound
}

func (mock mockStore) Put(session Session) error {
	session.touch()
	mock.session[session.Key] = session
	return nil
}

func (mock mockStore) Erase(key Key) error {
	delete(mock.session, key)
	return nil
}

func (mock mockStore) UpdateExpiration(sessions ...Session) error {
	for _, s := range sessions {
		s.touch()
		mock.session[s.Key] = s
	}

	return nil
}
func (mock mockStore) CollectExpired() error {
	var expire = time.Now().Add(-mock.expire)
	for key, session := range mock.session {
		if session.access.Before(expire) {
			delete(mock.session, key)
		}
	}

	return nil
}

// Stores nothing
type nullStore struct{}

func (_ nullStore) Retrieve(_ Key) (any, error)         { return nil, NotFound }
func (_ nullStore) Put(_ Session) error                 { return nil }
func (_ nullStore) Erase(_ Key) error                   { return nil }
func (_ nullStore) UpdateExpiration(_ ...Session) error { return nil }
func (_ nullStore) CollectExpired() error               { return nil }

func keeperHasKey(keeper *Keeper, key Key) bool {
	_, ok := keeper.session[key]
	return ok
}

func expectValue(t *testing.T, keeper *Keeper, key Key, value any) {
	v, err := keeper.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if v != value {
		t.Fatalf("keeper.Get gives wrong value '%v' wants '%v'", v, value)
	}
}

func TestKeeperGet(t *testing.T) {
	keeper := NewKeeper(nullStore{}, time.Second)

	k1, _ := keeper.Keep(1)
	k2, _ := keeper.Keep(2)

	expectValue(t, keeper, k1, 1)
	expectValue(t, keeper, k2, 2)
}

func TestKeeperGC(t *testing.T) {
	keeper := NewKeeper(nullStore{}, time.Millisecond/2)

	k1, _ := keeper.Keep(1)
	time.Sleep(time.Millisecond)

	k2, _ := keeper.Keep(2)
	keeper.gc()

	_, err := keeper.Get(k1)
	if err == nil {
		t.Fatal("expected keeper to return NotFound error")
	}

	expectValue(t, keeper, k2, 2)
}

func TestKeeperStore(t *testing.T) {
	mock := newMockStore(time.Millisecond * 4)
	keeper := NewKeeper(mock, time.Millisecond/2)

	k1, _ := keeper.Keep(1)

	// Offload to store
	time.Sleep(time.Millisecond)
	keeper.gc()

	// Retrieved from store
	expectValue(t, keeper, k1, 1)

	// Offload to store
	time.Sleep(time.Millisecond)
	keeper.gc()

	// Expire the store
	time.Sleep(time.Millisecond * 5)
	keeper.gc()

	// Should not be available anymore
	_, err := keeper.Get(k1)
	if err != NotFound {
		t.Fatal("expected keeper.Get to return NotFound error")
	}
}
