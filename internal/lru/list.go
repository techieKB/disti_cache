package lru

import "time"

type Node struct {
	key       string
	value     string
	expiresAt time.Time // zero = never expires
	prev      *Node
	next      *Node
}

type List struct {
	head Node
	tail Node
	len  int
}

func NewList() *List {
	l := &List{}
	l.head.next = &l.tail
	l.tail.prev = &l.head
	return l
}

func (l *List) PushFront(key, value string) *Node {
	n := &Node{key: key, value: value}
	l.insertFront(n)
	return n
}

// MoveToFront marks n as most recently used.
func (l *List) MoveToFront(n *Node) {
	l.unlink(n)
	l.insertFront(n)
}

// Remove unlinks n from the list.
func (l *List) Remove(n *Node) {
	l.unlink(n)
}

// Back returns the least recently used node, or nil if the list is empty.
func (l *List) Back() *Node {
	if l.len == 0 {
		return nil
	}
	return l.tail.prev
}

func (l *List) Len() int { return l.len }

func (l *List) insertFront(n *Node) {
	n.prev = &l.head
	n.next = l.head.next
	l.head.next.prev = n
	l.head.next = n
	l.len++
}

func (l *List) unlink(n *Node) {
	n.prev.next = n.next
	n.next.prev = n.prev
	n.prev, n.next = nil, nil // drop stale links so misuse fails loudly
	l.len--
}
