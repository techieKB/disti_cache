package lru

import "testing"

func mustGet(t *testing.T, c *Cache, key, want string) {
	t.Helper()
	got, ok := c.Get(key)
	if !ok || got != want {
		t.Fatalf("Get(%q) = %q, %v; want %q, true", key, got, ok, want)
	}
}

func mustMiss(t *testing.T, c *Cache, key string) {
	t.Helper()
	if got, ok := c.Get(key); ok {
		t.Fatalf("Get(%q) = %q, true; want miss", key, got)
	}
}

func TestGetMissing(t *testing.T) {
	mustMiss(t, New(2), "nope")
}

func TestSetThenGet(t *testing.T) {
	c := New(2)
	c.Set("a", "1")
	mustGet(t, c, "a", "1")
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := New(2)
	c.Set("a", "1")
	c.Set("b", "2")
	mustGet(t, c, "a", "1") // a is now most recent, so b is the LRU entry
	c.Set("c", "3")
	mustMiss(t, c, "b")
	mustGet(t, c, "a", "1")
	mustGet(t, c, "c", "3")
	if c.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", c.Len())
	}
}

func TestSetExistingUpdatesAndPromotes(t *testing.T) {
	c := New(2)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("a", "9") // update must also promote a
	c.Set("c", "3")
	mustMiss(t, c, "b")
	mustGet(t, c, "a", "9")
}

func TestDelete(t *testing.T) {
	c := New(2)
	c.Set("a", "1")
	if !c.Delete("a") {
		t.Fatal("Delete(a) = false, want true")
	}
	if c.Delete("a") {
		t.Fatal("second Delete(a) = true, want false")
	}
	mustMiss(t, c, "a")
	if c.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", c.Len())
	}
}

func TestCapacityOne(t *testing.T) {
	c := New(1)
	c.Set("a", "1")
	c.Set("b", "2")
	mustMiss(t, c, "a")
	mustGet(t, c, "b", "2")
}

func TestNewPanicsOnNonPositiveCapacity(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New(0) did not panic")
		}
	}()
	New(0)
}
