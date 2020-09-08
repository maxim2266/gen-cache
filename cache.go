package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type K = string
type V = string

type Fn = func(K) (V, error)

type Cache struct {
	mu    sync.Mutex
	cache map[K]*CacheNode
	lru   *CacheNode

	size  int
	ttl   time.Duration
	fetch Fn
}

type CacheNode struct {
	key   K
	value V
	err   error
	ts    time.Time

	once       sync.Once
	prev, next *CacheNode
}

func CacheFunc(n int, ttl time.Duration, fn Fn) Fn {
	return New(n, ttl, fn).Get
}

func New(size int, ttl time.Duration, fetch Fn) *Cache {
	if size < 2 || size > 16*1024*1024 {
		panic(fmt.Errorf("attempted to create a cache (K -> V) with invalid capacity of %d items", size))
	}

	if fetch == nil {
		panic(errors.New("attempted to create a cache (K -> V) with null fetch() function"))
	}

	return &Cache{
		cache: make(map[K]*CacheNode, size),
		size:  size,
		ttl:   ttl,
		fetch: fetch,
	}
}

func (c *Cache) Get(key K) (V, error) {
	node := c.get(key)

	node.once.Do(func() {
		defer func() {
			if p := recover(); p != nil {
				node.err = fmt.Errorf("panic: %+v", p)
				panic(p)
			}
		}()

		node.value, node.err = c.fetch(key)
	})

	return node.value, node.err
}

func (c *Cache) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node := c.cache[key]; node != nil {
		c.deleteNode(node)
	}
}

func (c *Cache) get(key K) (node *CacheNode) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node = c.cache[key]; node != nil { // found
		c.lruRemove(node)

		if time.Since(node.ts) > c.ttl {
			node.next, node.prev = nil, nil // help gc
			node = c.newNode(node.key)
		}
	} else { // not found
		if len(c.cache) == c.size { // cache full
			c.deleteNode(c.lru)
		}

		node = c.newNode(key)
	}

	c.lruAdd(node)
	return
}

func (c *Cache) newNode(key K) (node *CacheNode) {
	node = &CacheNode{
		key: key,
		ts:  time.Now(),
	}

	c.cache[key] = node
	return
}

func (c *Cache) deleteNode(node *CacheNode) {
	c.lruRemove(node)
	node.next, node.prev = nil, nil // help gc
	delete(c.cache, node.key)
}

func (c *Cache) lruRemove(node *CacheNode) {
	if node.next == node.prev {
		c.lru = nil
	} else {
		if c.lru == node {
			c.lru = node.prev
		}

		node.prev.next, node.next.prev = node.next, node.prev
	}
}

func (c *Cache) lruAdd(node *CacheNode) {
	if c.lru == nil {
		c.lru = node
		node.next, node.prev = node, node
	} else {
		node.next, node.prev = c.lru.next, c.lru.prev
		node.next.prev, node.prev.next = node, node
		c.lru = node.prev // bubble up
	}
}
