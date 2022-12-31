package cache

import (
	"context"
	"testing"
	"time"
)

func TestGenericCacheSet(t *testing.T) {
	cache := NewGenericCache[int, string]("", time.Millisecond)

	cache.Set(1, "Hello")
	cache.Set(2, "world")
	cache.Set(2, "World")

	v1, _ := cache.Get(1)
	v2, _ := cache.Get(2)

	if v1 != "Hello" {
		t.Fatalf("incorrect value in cache, expected '%s' got '%s'", v1, "Hello")
	}

	if v2 != "World" {
		t.Fatalf("incorrect value in cache, expected '%s' got '%s'", v2, "World")
	}
}

func TestGenericCacheDel(t *testing.T) {
	cache := NewGenericCache[int, string]("", time.Millisecond)

	cache.Set(1, "Hello")
	cache.Set(2, "World")

	cache.Del(1)
	if len(cache.data) != 1 {
		t.Fatal("expected data length of 1")
	}
	cache.Del(2)
	if len(cache.data) != 0 {
		t.Fatal("expected data length of 0")
	}
}

func TestGenericGC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewGenericCache[int, string]("", time.Millisecond*100)
	go cache.GC(ctx)

	cache.Set(1, "Hello")
	cache.Set(2, "World")
	time.Sleep(time.Millisecond * 50)
	cache.Set(3, "!!!")
	cache.Get(2)

	_, ok := cache.data[1]
	if !ok {
		t.Fatal("expected data 1")
	}
	_, ok = cache.data[2]
	if !ok {
		t.Fatal("expected data 2")
	}

	time.Sleep(time.Millisecond * 55)

	_, ok = cache.data[1]
	if ok {
		t.Fatal("unexpected data 1")
	}

	_, ok = cache.data[2]
	if !ok {
		t.Fatal("expected data 2")
	}
}
