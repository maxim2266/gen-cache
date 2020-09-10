package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
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

func TestSimpleOperations(t *testing.T) {
	const size = 10

	var backend intBackend

	cache := New(size, time.Hour, backend.fn)
	cont, err := fillN(cache, size)

	if err != nil {
		t.Error(err)
		return
	}

	// LRU: { 0 ... 9 }

	// get existing
	if _, err = cache.Get(0); err != nil {
		t.Errorf("unexpected error: %w", err)
		t.Log(dumpLRU(cache))
		return
	}

	// LRU: { 1 ... 9, 0 }
	cont = append(cont[1:], cont[0])

	if err = matchContent("after Get(0)", cache, cont); err != nil {
		t.Error(err)
		t.Log(dumpLRU(cache))
		return
	}

	// delete existing
	cache.Delete(0)

	// LRU: { 1 ... 9 }
	cont = cont[:len(cont)-1]

	if err = matchContent("after Delete(0)", cache, cont); err != nil {
		t.Error(err)
		t.Log(dumpLRU(cache))
		return
	}

	// get two more
	if _, err = cache.Get(20); err != nil {
		t.Errorf("unexpected error: %w", err)
		t.Log(dumpLRU(cache))
		return
	}

	if _, err = cache.Get(21); err != nil {
		t.Errorf("unexpected error: %w", err)
		t.Log(dumpLRU(cache))
		return
	}

	// LRU: { 2 ... 9, 20, 21 }
	cont = append(cont[1:], C{key: 20, value: -20}, C{key: 21, value: -21})

	if err = matchContent("after Get(20), Get(21)", cache, cont); err != nil {
		t.Error(err)
		t.Log(dumpLRU(cache))
		return
	}

	// get from the middle of LRU
	if _, err = cache.Get(3); err != nil {
		t.Errorf("unexpected error: %w", err)
		t.Log(dumpLRU(cache))
		return
	}

	// LRU: { 2, 4 ... 9, 20, 21, 3 }
	cont = append(append([]C{cont[0]}, cont[2:]...), cont[1])

	if err = matchContent("after Get(3)", cache, cont); err != nil {
		t.Error(err)
		t.Log(dumpLRU(cache))
		return
	}

	// random shuffle
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(cont), func(i, j int) { cont[i], cont[j] = cont[j], cont[i] })

	for _, c := range cont {
		v, err := cache.Get(c.key)

		if err != nil {
			t.Errorf("unexpected error: %w", err)
			t.Log(dumpLRU(cache))
			return
		}

		if v != c.value {
			t.Errorf("value mismatch for key %d: %d instead of %d", c.key, v, c.value)
			return
		}
	}

	if err = matchContent("after random shuffle", cache, cont); err != nil {
		t.Error(err)
		t.Log(dumpLRU(cache))
		return
	}

	// delete in random order
	rand.Shuffle(len(cont), func(i, j int) { cont[i], cont[j] = cont[j], cont[i] })

	for _, c := range cont {
		cache.Delete(c.key)
	}

	if err = assertEmpty("after deleting all records", cache); err != nil {
		t.Error(err)
		return
	}

	// number of backend calls
	const exp = 12

	if backend.calls != exp {
		t.Errorf("unexpected number of backend calls: %d instead of %d", backend.calls, exp)
		return
	}
}

func TestConcurrentAccess(t *testing.T) {
	const threads = 20

	var (
		backend intBackendMT
		wg      sync.WaitGroup
		calls   uint64
	)

	cache := New(100, time.Millisecond, backend.fn)

	get := func(k int) (int, error) {
		atomic.AddUint64(&calls, 1)
		return cache.Get(k)
	}

	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()

			var keys [1000]int

			for i := 0; i < len(keys); i++ {
				keys[i] = i % 105 // 5% chance of error for the current backend
			}

			ts := time.Now()

			for time.Since(ts) < 500*time.Millisecond {
				rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })

				for _, k := range keys {
					v, err := get(k)

					if backend.validKey(k) {
						if err != nil {
							t.Errorf("unexpected error: %w", err)
							return
						}

						if v != -k {
							t.Errorf("value mismatch for key %d: %d instead of %d", k, v, -k)
							return
						}
					} else if err == nil {
						t.Errorf("missing error for key %d", k)
						return
					}
				}
			}
		}()
	}

	wg.Wait()

	// cache efficiency
	backCalls := backend.hit + backend.miss
	ratio := 100 * float32(calls-backCalls) / float32(calls)

	t.Logf("cache efficiency %.2f%%", ratio)
}
