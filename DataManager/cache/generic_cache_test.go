package cache

import (
	"context"
	"testing"
	"time"
)

func TestGenericCacheSet(t *testing.T) {
	cache := NewGeneric[int, string]("", time.Millisecond, 0)

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
	cache := NewGeneric[int, string]("", time.Millisecond, 0)

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

	cache := NewGeneric[int, string]("", time.Millisecond*10, time.Millisecond*20)
	go cache.GC(ctx)

	cache.Set(1, "Hello")
	cache.Set(2, "World")
	time.Sleep(time.Millisecond * 5)
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

	time.Sleep(time.Millisecond * 10)

	_, ok = cache.data[1]
	if ok {
		t.Fatal("unexpected data 1")
	}

	_, ok = cache.data[2]
	if !ok {
		t.Fatal("expected data 2")
	}

	cache.Get(2)
	time.Sleep(time.Millisecond * 5)
	cache.Get(2)
	time.Sleep(time.Millisecond * 5)

	_, ok = cache.data[2]
	if ok {
		t.Fatal("unexpected data 2")
	}
}
