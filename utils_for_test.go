package main

import (
	"fmt"
	"strings"
)

// backend with hit/miss counters
type intBackend struct {
	calls int
	trace []int
}

func (b *intBackend) fn(key int) (int, error) {
	b.calls++

	if key >= 0 && key < 100 {
		b.trace = append(b.trace, key)
		return -key, nil
	}

	return 0, fmt.Errorf("key not found: %d", key)
}

func (b *intBackend) clear() {
	b.calls = 0
	b.trace = b.trace[:0]
}

func matchTraces(prefix string, got, exp []int) error {
	if len(got) != len(exp) {
		return fmt.Errorf("%s: trace mismatch: %v instead of %v", prefix, got, exp)
	}

	for i, v := range got {
		if v != exp[i] {
			return fmt.Errorf("%s: trace mismatch @ %d: %d instead of %d", prefix, i, v, exp[i])
		}
	}

	return nil
}

type C struct {
	key   int
	err   bool
	value int
}

func matchContent(prefix string, cache *Cache, cont []C) error {
	// initial checks
	if len(cache.cache) != len(cont) {
		return fmt.Errorf("%s: unexpected size of cache map: %d instead of %d",
			prefix, len(cache.cache), len(cont))
	}

	if len(cont) == 0 {
		if cache.lru != nil {
			return fmt.Errorf("%s: unexpected content in LRU: key %d", prefix, cache.lru.key)
		}

		return nil
	}

	// validate content
	lnode := cache.lru

	for i, c := range cont {
		// cache map validation
		node, found := cache.cache[c.key]

		if !found {
			return fmt.Errorf("%s: key %d not found in cache", prefix, c.key)
		}

		if node == nil {
			return fmt.Errorf("%s: nil node for key %d", prefix, c.key)
		}

		if err := checkNode(prefix+": cache map", &c, node); err != nil {
			return err
		}

		// LRU node validation
		if err := checkNode(prefix+": LRU", &c, lnode); err != nil {
			return err
		}

		// check LRU node pointers
		if lnode.next.prev != lnode || lnode.prev.next != lnode {
			return fmt.Errorf("%s: invalid LRU node connections (key %d)", prefix, lnode.key)
		}

		// prev LRU node
		if lnode = lnode.prev; lnode == cache.lru && i != len(cont)-1 {
			return fmt.Errorf("%s: LRU list ends at key %d", prefix, c.key)
		}
	}

	if lnode != cache.lru {
		return fmt.Errorf("%s: LRU list continues with key %d", prefix, lnode.key)
	}

	return nil
}

func checkNode(prefix string, c *C, node *CacheNode) error {
	if node.key != c.key {
		return fmt.Errorf("%s: key mismatch: %d instead of %d", prefix, node.key, c.key)
	}

	if c.err && node.err == nil {
		return fmt.Errorf("%s: missing error for key %d", prefix, c.key)
	} else if !c.err && node.value != c.value {
		return fmt.Errorf("%s: unexpected value for key %d: %c instead of %c",
			prefix, c.key, node.value, c.value)
	}

	return nil
}

func dumpLRU(cache *Cache) string {
	if cache.lru == nil {
		return "(empty)"
	}

	var res strings.Builder

	for node := cache.lru; ; {
		fmt.Fprintf(&res,
			"\n{ key: %v, value: %v, error: %v }",
			node.key, node.value, node.err)

		if node = node.prev; node == cache.lru {
			break
		}
	}

	return res.String()
}

func getOne(prefix string, cache *Cache, key int) error {
	value, err := cache.Get(key)

	if err != nil {
		return fmt.Errorf("%s: unexpected error: %w", prefix, err)
	}

	v := -key

	if value != v {
		return fmt.Errorf("%s: unexpected value: %d instead of %d", prefix, value, v)
	}

	// internals
	if cache.lru == nil {
		return fmt.Errorf("%s: nil LRU pointer", prefix)
	}

	if cache.lru.next == nil || !(cache.lru.next == cache.lru.prev && cache.lru.next == cache.lru) {
		return fmt.Errorf("%s: invalid LRU list", prefix)
	}

	if len(cache.cache) != 1 {
		return fmt.Errorf("%s: unexpected cache size: %d", prefix, len(cache.cache))
	}

	node, ok := cache.cache[key]

	if !ok {
		return fmt.Errorf("%s: missing cache record for key %d", prefix, key)
	}

	if node == nil {
		return fmt.Errorf("%s: nil node for key %d", prefix, key)
	}

	if node.value != v {
		return fmt.Errorf("%s: unexpected cache content: %d instead of %d", prefix, node.value, v)
	}

	return nil
}

func assertEmpty(prefix string, cache *Cache) error {
	if cache.lru != nil {
		return fmt.Errorf("%s: non-null LRU pointer", prefix)
	}

	if len(cache.cache) != 0 {
		return fmt.Errorf("%s: unexpected cache size: %d", prefix, len(cache.cache))
	}

	return nil
}
