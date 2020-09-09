package main

import (
	"fmt"
	"testing"
	"time"
)

func TestOneRecord(t *testing.T) {
	var backend intBackend

	cache := New(5, time.Hour, backend.fn)

	var err error

	// initial state
	if err = assertEmpty("initial state", cache); err != nil {
		t.Error(err)
		return
	}

	// insert one item and check
	prefix := "first insert"

	if err = getOne(prefix, cache, 5); err != nil {
		t.Error(err)
		return
	}

	if len(backend.trace) != 1 || backend.trace[0] != 5 {
		t.Errorf("%s: unexpected backtrace: %v", prefix, backend.trace)
		return
	}

	// read again
	prefix = "second read"

	if err = getOne(prefix, cache, 5); err != nil {
		t.Error(err)
		return
	}

	if len(backend.trace) != 1 || backend.trace[0] != 5 {
		t.Errorf("%s: unexpected backtrace: %v", prefix, backend.trace)
		return
	}

	if backend.calls != 1 {
		t.Errorf("%s: unexpected number of backend calls: %d", prefix, backend.calls)
	}

	// read nonexistent key
	prefix = "read nonexistent"

	const k = 1000

	_, err = cache.Get(k)

	if err == nil {
		t.Errorf("%s: missing error message", prefix)
		return
	}

	cont := [...]C{
		{key: 5, value: -5},
		{key: k, err: true},
	}

	if err = matchContent(prefix, cache, cont[:]); err != nil {
		t.Error(err)
		return
	}

	if len(cache.cache) != 2 || cache.lru == nil {
		t.Errorf("%s: invalid cache state: size %d, LRU empty: %v",
			prefix, len(cache.cache), cache.lru == nil)
		return
	}

	if cache.lru.next == cache.lru.prev && cache.lru.next == cache.lru {
		t.Errorf("%s: invalid cache state: LRU has only one node", prefix)
		return
	}

	// delete and read again
	cache.Delete(k)
	cache.Delete(5)

	if err = assertEmpty("after two deletes", cache); err != nil {
		t.Error(err)
		return
	}

	prefix = "second read"

	if err = getOne(prefix, cache, 5); err != nil {
		t.Error(err)
		return
	}

	exp := []int{5, 5}

	if err = matchTraces(prefix, backend.trace, exp); err != nil {
		t.Errorf("%s: unexpected backtrace: %v instead of %v", prefix, backend.trace, exp)
		return
	}
}

func TestFill(t *testing.T) {
	const size = 10

	var backend intBackend

	cache := New(size, time.Hour, backend.fn)
	cont, err := fillN(cache, size)

	if err != nil {
		t.Error(err)
		return
	}

	if err = matchContent("cache content", cache, cont); err != nil {
		t.Error(err)
		t.Log(dumpLRU(cache))
		return
	}

	if len(backend.trace) != size {
		t.Errorf("unexpected trace size: %d instead of %d", len(backend.trace), size)
		t.Log(backend.trace)
		return
	}

	for i, k := range backend.trace {
		if k != i {
			t.Errorf("unexpected trace key @ %d: %d instead of %d", i, k, i)
			t.Log(backend.trace)
			return
		}
	}

	if backend.calls != size {
		t.Errorf("unexpected number of backend calls: %d instead of %d", backend.calls, size)
		return
	}
}

func fillN(cache *Cache, N int) ([]C, error) {
	res := make([]C, 0, N)

	for i := 0; i < N; i++ {
		v, err := cache.Get(i)

		if err != nil {
			return nil, fmt.Errorf("unexpected error @ %d: %w", i, err)
		}

		if v != -i {
			return nil, fmt.Errorf("value mismatch @ %d: %d instead of %d", i, v, -i)
		}

		res = append(res, C{key: i, value: v})
	}

	return res, nil
}
