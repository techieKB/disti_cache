package ring

import (
	"crypto/md5"
	"encoding/binary"
	"slices"
	"strconv"
	"sync"
)

// Ring is read-heavy and Get never mutates, so an RWMutex fits here
// (unlike the LRU, whose reads reorder its list).
type Ring struct {
	mu     sync.RWMutex
	vnodes int
	hashes []uint64          // sorted vnode positions
	owner  map[uint64]string // vnode position -> node
	nodes  map[string]struct{}
}

func New(vnodes int) *Ring {
	if vnodes < 1 {
		panic("ring: vnodes must be >= 1")
	}
	return &Ring{vnodes: vnodes, owner: map[uint64]string{}, nodes: map[string]struct{}{}}
}

// hashKey must be identical in every process, or nodes disagree on owners.
// md5 is not for security here, just a well-mixed, stable, seedless hash.
func hashKey(s string) uint64 {
	sum := md5.Sum([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}

func (r *Ring) Add(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.nodes[node]; ok {
		return
	}
	r.nodes[node] = struct{}{}
	for i := range r.vnodes {
		h := hashKey(node + "#" + strconv.Itoa(i))
		if _, taken := r.owner[h]; taken {
			continue // 64-bit collision: astronomically rare, skip the point
		}
		r.owner[h] = node
		r.hashes = append(r.hashes, h)
	}
	slices.Sort(r.hashes)
}

func (r *Ring) Remove(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.nodes[node]; !ok {
		return
	}
	delete(r.nodes, node)
	kept := r.hashes[:0]
	for _, h := range r.hashes {
		if r.owner[h] == node {
			delete(r.owner, h)
			continue
		}
		kept = append(kept, h)
	}
	r.hashes = kept
}

func (r *Ring) Get(key string) string {
	if n := r.GetN(key, 1); len(n) > 0 {
		return n[0]
	}
	return ""
}

// GetN walks clockwise from key's position and returns up to n distinct nodes:
// the owner first, then the replicas.
func (r *Ring) GetN(key string, n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.hashes) == 0 || n <= 0 {
		return nil
	}
	n = min(n, len(r.nodes))
	start, _ := slices.BinarySearch(r.hashes, hashKey(key)) // first vnode >= hash
	out := make([]string, 0, n)
	for i := 0; len(out) < n; i++ {
		node := r.owner[r.hashes[(start+i)%len(r.hashes)]]
		if !slices.Contains(out, node) { // n is tiny (RF), linear scan beats a map
			out = append(out, node)
		}
	}
	return out
}

func (r *Ring) Has(node string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.nodes[node]
	return ok
}

func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.nodes))
	for n := range r.nodes {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}
