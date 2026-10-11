package lru

import (
	"slices"
	"testing"
)

// keys walks front-to-back AND back-to-front, so a broken prev or next
// pointer shows up as a mismatch.
func keys(t *testing.T, l *List) []string {
	t.Helper()
	var fwd, back []string
	for n := l.head.next; n != &l.tail; n = n.next {
		fwd = append(fwd, n.key)
	}
	for n := l.tail.prev; n != &l.head; n = n.prev {
		back = append(back, n.key)
	}
	slices.Reverse(back)
	if !slices.Equal(fwd, back) {
		t.Fatalf("links inconsistent: forward %v, backward %v", fwd, back)
	}
	if len(fwd) != l.Len() {
		t.Fatalf("Len() = %d, but list holds %d nodes", l.Len(), len(fwd))
	}
	return fwd
}

func wantKeys(t *testing.T, l *List, want ...string) {
	t.Helper()
	if got := keys(t, l); !slices.Equal(got, want) {
		t.Fatalf("list = %v, want %v", got, want)
	}
}

func TestEmptyList(t *testing.T) {
	l := NewList()
	if l.Back() != nil {
		t.Fatal("Back() on empty list should be nil")
	}
	wantKeys(t, l)
}

func TestPushFrontOrder(t *testing.T) {
	l := NewList()
	l.PushFront("a", "1")
	l.PushFront("b", "2")
	l.PushFront("c", "3")
	wantKeys(t, l, "c", "b", "a")
	if l.Back().key != "a" {
		t.Fatalf("Back() = %q, want a", l.Back().key)
	}
}

// Regression for the original bug: PushFront never set prev, so removing
// the back node wiped the whole list.
func TestRemoveBackKeepsRest(t *testing.T) {
	l := NewList()
	l.PushFront("a", "1")
	l.PushFront("b", "2")
	l.Remove(l.Back())
	wantKeys(t, l, "b")
}

func TestRemoveMiddleFrontAndLast(t *testing.T) {
	l := NewList()
	a := l.PushFront("a", "1")
	b := l.PushFront("b", "2")
	c := l.PushFront("c", "3")
	l.Remove(b)
	wantKeys(t, l, "c", "a")
	l.Remove(c)
	wantKeys(t, l, "a")
	l.Remove(a)
	wantKeys(t, l)
	if l.Back() != nil {
		t.Fatal("Back() should be nil after removing everything")
	}
}

func TestMoveToFront(t *testing.T) {
	l := NewList()
	a := l.PushFront("a", "1")
	l.PushFront("b", "2")
	c := l.PushFront("c", "3")
	l.MoveToFront(a) // from back
	wantKeys(t, l, "a", "c", "b")
	l.MoveToFront(a) // already front: no-op
	wantKeys(t, l, "a", "c", "b")
	l.MoveToFront(c) // from middle
	wantKeys(t, l, "c", "a", "b")
}
