package ring

import (
	"strconv"
	"sync"
	"testing"
)

func testKeys(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "key-" + strconv.Itoa(i)
	}
	return out
}

func newRing(nodes ...string) *Ring {
	r := New(150)
	for _, n := range nodes {
		r.Add(n)
	}
	return r
}

func TestEmptyRing(t *testing.T) {
	r := New(10)
	if got := r.Get("k"); got != "" {
		t.Fatalf("Get on empty ring = %q", got)
	}
	if got := r.GetN("k", 3); len(got) != 0 {
		t.Fatalf("GetN on empty ring = %v", got)
	}
}

func TestRingIsDeterministicAcrossInsertOrder(t *testing.T) {
	a := newRing("n1", "n2", "n3")
	b := newRing("n3", "n1", "n2")
	for _, k := range testKeys(1000) {
		if a.Get(k) != b.Get(k) {
			t.Fatalf("rings disagree on %q: %s vs %s", k, a.Get(k), b.Get(k))
		}
	}
}

func TestBalance(t *testing.T) {
	r := newRing("n1", "n2", "n3")
	counts := map[string]int{}
	keys := testKeys(30_000)
	for _, k := range keys {
		counts[r.Get(k)]++
	}
	for node, c := range counts {
		share := float64(c) / float64(len(keys))
		if share < 0.25 || share > 0.42 {
			t.Errorf("%s owns %.1f%% of keys, want ~33%%", node, share*100)
		}
	}
}

func TestAddMovesKeysOnlyToNewNode(t *testing.T) {
	r := newRing("n1", "n2", "n3")
	keys := testKeys(10_000)
	before := map[string]string{}
	for _, k := range keys {
		before[k] = r.Get(k)
	}
	r.Add("n4")
	moved := 0
	for _, k := range keys {
		if after := r.Get(k); after != before[k] {
			moved++
			if after != "n4" {
				t.Fatalf("%q moved %s -> %s; keys may only move to the new node", k, before[k], after)
			}
		}
	}
	if frac := float64(moved) / float64(len(keys)); frac < 0.15 || frac > 0.35 {
		t.Fatalf("%.1f%% of keys moved, want ~25%%", frac*100)
	}
}

func TestRemoveMovesOnlyRemovedNodesKeys(t *testing.T) {
	r := newRing("n1", "n2", "n3", "n4")
	keys := testKeys(10_000)
	before := map[string]string{}
	for _, k := range keys {
		before[k] = r.Get(k)
	}
	r.Remove("n2")
	for _, k := range keys {
		if before[k] != "n2" && r.Get(k) != before[k] {
			t.Fatalf("%q moved %s -> %s although its owner stayed", k, before[k], r.Get(k))
		}
		if r.Get(k) == "n2" {
			t.Fatalf("%q still maps to removed node", k)
		}
	}
	if r.Has("n2") || len(r.Nodes()) != 3 {
		t.Fatalf("Nodes() = %v after removing n2", r.Nodes())
	}
}

func TestGetNReturnsDistinctNodes(t *testing.T) {
	r := newRing("n1", "n2", "n3")
	for _, k := range testKeys(500) {
		got := r.GetN(k, 2)
		if len(got) != 2 || got[0] == got[1] || got[0] != r.Get(k) {
			t.Fatalf("GetN(%q, 2) = %v (Get = %s)", k, got, r.Get(k))
		}
	}
	if got := r.GetN("k", 5); len(got) != 3 {
		t.Fatalf("GetN(k, 5) on 3 nodes = %v, want all 3", got)
	}
}

func TestAddIsIdempotent(t *testing.T) {
	r := New(10)
	r.Add("n1")
	r.Add("n1")
	if len(r.hashes) != 10 || len(r.Nodes()) != 1 {
		t.Fatalf("double Add: %d vnodes, nodes=%v", len(r.hashes), r.Nodes())
	}
}

func TestConcurrentReadsAndMembershipChanges(t *testing.T) {
	r := newRing("n1", "n2")
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for _, k := range testKeys(2000) {
				r.GetN(k, 2)
			}
		})
	}
	wg.Go(func() {
		for range 50 {
			r.Add("n3")
			r.Remove("n3")
		}
	})
	wg.Wait()
}
