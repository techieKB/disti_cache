package lru

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time          { return f.t }
func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func newFake() (*Cache, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1_000, 0)}
	return NewWithClock(10, clk.Now), clk
}

func TestTTLExpiresLazily(t *testing.T) {
	c, clk := newFake()
	c.SetWithTTL("a", "1", time.Second)
	clk.Advance(999 * time.Millisecond)
	mustGet(t, c, "a", "1")
	clk.Advance(time.Millisecond) // exactly at expiry counts as expired
	mustMiss(t, c, "a")
	if c.Len() != 0 {
		t.Fatalf("expired key not removed on Get; Len() = %d", c.Len())
	}
}

func TestZeroTTLNeverExpires(t *testing.T) {
	c, clk := newFake()
	c.SetWithTTL("a", "1", 0)
	clk.Advance(24 * time.Hour)
	mustGet(t, c, "a", "1")
}

func TestSetClearsPreviousTTL(t *testing.T) {
	c, clk := newFake()
	c.SetWithTTL("a", "1", time.Second)
	c.Set("a", "2")
	clk.Advance(time.Hour)
	mustGet(t, c, "a", "2")
}

func TestPurgeExpired(t *testing.T) {
	c, clk := newFake()
	c.SetWithTTL("a", "1", time.Second)
	c.SetWithTTL("b", "2", 5*time.Second)
	c.Set("c", "3")
	clk.Advance(2 * time.Second)
	if n := c.PurgeExpired(); n != 1 {
		t.Fatalf("PurgeExpired() = %d, want 1", n)
	}
	if c.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", c.Len())
	}
}

// Regression: purge must remove from the list too, or a ghost node later
// evicts the map entry of a re-set key.
func TestPurgeThenResetDoesNotCorrupt(t *testing.T) {
	c, clk := newFake()
	c.SetWithTTL("a", "1", time.Second)
	clk.Advance(2 * time.Second)
	c.PurgeExpired()
	c.Set("a", "2")
	if c.Len() != 1 {
		t.Fatalf("Len() = %d after purge+reset, want 1 (ghost node left in list?)", c.Len())
	}
	mustGet(t, c, "a", "2")
}

func TestSetWithTTLOnExistingKeyUpdatesExpiry(t *testing.T) {
	c, clk := newFake()
	c.SetWithTTL("a", "1", time.Second)
	c.SetWithTTL("a", "2", time.Hour)
	clk.Advance(2 * time.Second)
	mustGet(t, c, "a", "2")
}
