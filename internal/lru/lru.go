package lru

type Cache struct {
	capacity int
	ll       *List            // front = most recently used
	items    map[string]*Node // key -> node in ll
}

func New(capacity int) *Cache {
	if capacity < 1 {
		panic("lru: capacity must be >= 1")
	}
	return &Cache{
		capacity: capacity,
		ll:       NewList(),
		items:    make(map[string]*Node),
	}
}

func (c *Cache) Get(key string) (string, bool) {
	n, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.ll.MoveToFront(n)
	return n.value, true
}

func (c *Cache) Set(key, value string) {
	if n, ok := c.items[key]; ok {
		n.value = value
		c.ll.MoveToFront(n)
		return
	}
	c.items[key] = c.ll.PushFront(key, value)
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

func (c *Cache) Len() int { return c.ll.Len() }

func (c *Cache) removeNode(n *Node) {
	c.ll.Remove(n)
	delete(c.items, n.key)
}
