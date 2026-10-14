// Package lru implements a fixed-capacity least-recently-used cache with
// optional per-key TTL. It is NOT safe for concurrent use; callers must
// synchronize (see package store).
package lru

import "time"

type Cache struct {
	capacity int
	ll       *List            // front = most recently used
	items    map[string]*Node // key -> node in ll
	now      func() time.Time
}

func New(capacity int) *Cache { return NewWithClock(capacity, time.Now) }

// NewWithClock lets tests control time instead of sleeping.
func NewWithClock(capacity int, now func() time.Time) *Cache {
	if capacity < 1 {
		panic("lru: capacity must be >= 1")
	}
	return &Cache{
		capacity: capacity,
		ll:       NewList(),
		items:    make(map[string]*Node),
		now:      now,
	}
}

func (c *Cache) Get(key string) (string, bool) {
	n, ok := c.items[key]
	if !ok {
		return "", false
	}
	if c.expired(n) { // lazy expiry: check only the key being read, O(1)
		c.removeNode(n)
		return "", false
	}
	c.ll.MoveToFront(n)
	return n.value, true
}

func (c *Cache) Set(key, value string) { c.SetWithTTL(key, value, 0) }

// SetWithTTL stores value; ttl <= 0 means the key never expires.
// Overwriting a key always replaces its TTL (Set clears it).
func (c *Cache) SetWithTTL(key, value string, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = c.now().Add(ttl)
	}
	if n, ok := c.items[key]; ok {
		n.value, n.expiresAt = value, exp
		c.ll.MoveToFront(n)
		return
	}
	n := c.ll.PushFront(key, value)
	n.expiresAt = exp
	c.items[key] = n
	if c.ll.Len() > c.capacity {
		c.removeNode(c.ll.Back())
	}
}

func (c *Cache) Delete(key string) bool {
	n, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeNode(n)
	return true
}

// Len counts entries still held, including expired ones not yet purged.
func (c *Cache) Len() int { return c.ll.Len() }

// PurgeExpired removes every expired entry and returns how many it removed.
// O(n): meant for a background janitor, never the read path.
func (c *Cache) PurgeExpired() int {
	removed := 0
	for _, n := range c.items { // deleting during range is safe in Go
		if c.expired(n) {
			c.removeNode(n) // list AND map, so they never disagree
			removed++
		}
	}
	return removed
}

func (c *Cache) expired(n *Node) bool {
	return !n.expiresAt.IsZero() && !c.now().Before(n.expiresAt)
}

func (c *Cache) removeNode(n *Node) {
	c.ll.Remove(n)
	delete(c.items, n.key)
}
