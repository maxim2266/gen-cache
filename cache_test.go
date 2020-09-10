package main

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOneRecord(t *testing.T) {
	var backend tracingBackend

	cache := New(5, time.Hour, backend.fn)

	if err := assertEmpty(cache); err != nil {
		t.Error("new cache is not empty:", err)
		return
	}

	// add one valid key
	v, err := cache.Get(5)

	if err != nil {
		t.Error("error inserting a key:", err)
		return
	}

	if v != -5 {
		t.Errorf("unexpected value: %d instead of -5", v)
	}

	if err = checkState(cache, []int{5}, backend.validKey); err != nil {
		t.Error("error after the first insert:", err)
		return
	}

	// delete the key
	cache.Delete(5)

	if err = assertEmpty(cache); err != nil {
		t.Error("error after deleting a key:", err)
		return
	}

	// try the same, but with invalid key
	_, err = cache.Get(1000)

	if err == nil {
		t.Error("missing error while inserting an invalid key")
		return
	}

	if err = checkState(cache, []int{1000}, backend.validKey); err != nil {
		t.Error("error after the first insert:", err)
		return
	}

	// delete the key
	cache.Delete(1000)

	if err = assertEmpty(cache); err != nil {
		t.Error("error after deleting a key:", err)
		return
	}

	// check trace
	if err = matchTraces(backend.trace, []int{5, 1000}); err != nil {
		t.Error("trace mismatch:", err)
		return
	}
}

func TestFewRecords(t *testing.T) {
	var (
		backend tracingBackend
		err     error
	)

	cache := New(2, time.Hour, backend.fn)

	if err = assertEmpty(cache); err != nil {
		t.Error("new cache is not empty:", err)
		return
	}

	if err = fill(cache.Get, []int{1, 2, 3}, backend.validKey); err != nil {
		t.Error("error filling the cache:", err)
		return
	}

	if err = checkState(cache, []int{2, 3}, backend.validKey); err != nil {
		t.Error("invalid state after fill:", err)
		t.Log(dumpLRU(cache))
		return
	}

	if err = matchTraces(backend.trace, []int{1, 2, 3}); err != nil {
		t.Error("trace mismatch:", err)
		return
	}
}

func TestCacheOperation(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	const cacheSize = 5

	var (
		backend tracingBackend
		err     error
	)

	cache := New(cacheSize, time.Hour, backend.fn)

	if err = fill(cache.Get, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, backend.validKey); err != nil {
		t.Error("error filling the cache:", err)
		return
	}

	// LRU: {5, 6, 7, 8, 9}
	if err = checkState(cache, []int{5, 6, 7, 8, 9}, backend.validKey); err != nil {
		t.Error("invalid cache state:", err)
		t.Log(dumpLRU(cache))
		return
	}

	if err = fill(cache.Get, []int{6, 7}, backend.validKey); err != nil {
		t.Error("error filling the cache:", err)
		return
	}

	// LRU: {5, 8, 9, 6, 7}
	if err = checkState(cache, []int{5, 8, 9, 6, 7}, backend.validKey); err != nil {
		t.Error("invalid cache state:", err)
		t.Log(dumpLRU(cache))
		return
	}

	if err = fill(cache.Get, []int{42, 9}, backend.validKey); err != nil {
		t.Error("error filling the cache:", err)
		return
	}

	// LRU: {8, 6, 7, 42, 9}
	if err = checkState(cache, []int{8, 6, 7, 42, 9}, backend.validKey); err != nil {
		t.Error("invalid cache state:", err)
		t.Log(dumpLRU(cache))
		return
	}

	cache.Delete(6)
	cache.Delete(8)
	cache.Delete(9)

	// LRU: {7, 42}
	if err = checkState(cache, []int{7, 42}, backend.validKey); err != nil {
		t.Error("invalid cache state:", err)
		t.Log(dumpLRU(cache))
		return
	}

	// check traces
	if err = matchTraces(backend.trace, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 42}); err != nil {
		t.Error("invalid trace:", err)
		t.Log(backend.trace)
		return
	}
}

func TestRandomFill(t *testing.T) {
	var (
		backend tracingBackend
		calls   int
	)

	const cacheSize = 90

	cache := New(cacheSize, time.Hour, backend.fn)
	get := func(k int) (int, error) {
		calls++
		return cache.Get(k)
	}

	var keys [500000]int

	for i := range keys {
		keys[i] = rand.Intn(100)
	}

	if err := fill(get, keys[:], backend.validKey); err != nil {
		t.Error("error filling the cache:", err)
		return
	}

	// calculate cache efficiency
	ratio := 100 * float64(calls-len(backend.trace)) / float64(calls)

	t.Logf("cache efficiency %.2f%%", ratio)

	exp := float64(cacheSize)

	if math.Abs((ratio-exp)/exp) > 0.01 {
		t.Errorf("cache efficiency: %.2f%% instead of %.2f%%", ratio, exp)
		return
	}
}

func TestConcurrentAccess(t *testing.T) {
	const (
		threads   = 1
		cacheSize = 90
	)

	var (
		backend intBackendMT
		wg      sync.WaitGroup
		calls   uint64
	)

	cache := New(cacheSize, 500*time.Microsecond, backend.fn)

	get := func(k int) (int, error) {
		atomic.AddUint64(&calls, 1)
		return cache.Get(k)
	}

	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()

			var keys [100000]int

			for i := range keys {
				keys[i] = rand.Intn(100)
			}

			ts := time.Now()

			for time.Since(ts) < time.Second {
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

	ratio := 100 * float64(calls-(backend.hit+backend.miss)) / float64(calls)
	t.Logf("cache efficiency %.2f%%", ratio)

	exp := float64(cacheSize)

	if math.Abs((ratio-exp)/exp) > 0.01 {
		t.Errorf("cache efficiency: %.2f%% instead of %.2f%%", ratio, exp)
		return
	}
}
