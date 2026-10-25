package store

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// clock is safe to read from the janitor goroutine while the test advances it.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestSetGetDelete(t *testing.T) {
	s := New(4, 10)
	s.Set("a", "1", 0)
	if v, ok := s.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a) = %q, %v", v, ok)
	}
	if !s.Delete("a") {
		t.Fatal("Delete(a) = false")
	}
	if _, ok := s.Get("a"); ok {
		t.Fatal("a still present after Delete")
	}
}

func TestStatsCountHitsAndMisses(t *testing.T) {
	s := New(4, 10)
	s.Set("a", "1", 0)
	s.Get("a")
	s.Get("a")
	s.Get("missing")
	st := s.Stats()
	if st.Hits != 2 || st.Misses != 1 || st.Keys != 1 {
		t.Fatalf("Stats() = %+v, want Hits=2 Misses=1 Keys=1", st)
	}
}

// Run with -race. Correctness is "no race reported + counters add up".
func TestConcurrentAccess(t *testing.T) {
	s := New(8, 100)
	const workers, ops = 32, 999
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := range ops {
				key := strconv.Itoa((w*ops + i) % 50)
				switch i % 3 {
				case 0:
					s.Set(key, "v", 0)
				case 1:
					s.Get(key)
				case 2:
					s.Delete(key)
				}
			}
		})
	}
	wg.Wait()
	st := s.Stats()
	if got, want := st.Hits+st.Misses, uint64(workers*ops/3); got != want {
		t.Fatalf("hits+misses = %d, want %d", got, want)
	}
}

func TestJanitorPurgesExpiredKeysAndStops(t *testing.T) {
	clk := &clock{t: time.Unix(1_000, 0)}
	s := NewWithClock(4, 100, clk.Now)
	s.Set("short", "1", time.Second)
	s.Set("forever", "2", 0)
	clk.Advance(2 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.RunJanitor(ctx, 5*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for s.Stats().Keys != 1 { // Stats does not trigger lazy expiry; only the janitor can get us here
		if time.Now().After(deadline) {
			t.Fatalf("janitor did not purge; Keys = %d", s.Stats().Keys)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunJanitor did not return after cancel (goroutine leak)")
	}
}

// go test -bench=Parallel -cpu=1,8 ./internal/store/
func BenchmarkStoreParallel(b *testing.B) {
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}
	for _, shards := range []int{1, 16} {
		b.Run(fmt.Sprintf("shards=%d", shards), func(b *testing.B) {
			s := New(shards, 1024)
			for _, k := range keys {
				s.Set(k, "v", 0)
			}
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					k := keys[i%len(keys)]
					if i%10 == 0 {
						s.Set(k, "v", 0)
					} else {
						s.Get(k)
					}
					i++
				}
			})
		})
	}
}
