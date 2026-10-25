// Package store is a sharded wrapper around lru.Cache. Each shard is an
// lru.Cache with its own lock, so goroutines only contend when their keys
// land in the same shard.
package store

import (
	"context"
	"hash/fnv"
	"sync/atomic"
	"time"

	"github.com/techieKB/disti_cache/internal/lru"
)

type Store struct {
	shards []*lru.Cache
	hits   atomic.Uint64
	misses atomic.Uint64
}

type Stats struct {
	Hits, Misses uint64
	Keys         int // includes expired keys the janitor has not purged yet
}

func New(numShards, capacityPerShard int) *Store {
	return NewWithClock(numShards, capacityPerShard, time.Now)
}

func NewWithClock(numShards, capacityPerShard int, now func() time.Time) *Store {
	if numShards < 1 {
		panic("store: numShards must be >= 1")
	}
	st := &Store{shards: make([]*lru.Cache, numShards)}
	for i := range st.shards {
		st.shards[i] = lru.NewWithClock(capacityPerShard, now)
	}
	return st
}

// shardFor maps a key to its shard. FNV-1a is fast and allocation-free,
// which matters because it runs on every operation.
func (st *Store) shardFor(key string) *lru.Cache {
	h := fnv.New32a()
	h.Write([]byte(key))
	return st.shards[h.Sum32()%uint32(len(st.shards))]
}

func (st *Store) Set(key, value string, ttl time.Duration) {
	st.shardFor(key).SetWithTTL(key, value, ttl)
}

func (st *Store) Get(key string) (string, bool) {
	v, ok := st.shardFor(key).Get(key)
	if ok {
		st.hits.Add(1)
	} else {
		st.misses.Add(1)
	}
	return v, ok
}

func (st *Store) Delete(key string) bool {
	return st.shardFor(key).Delete(key)
}

func (st *Store) Stats() Stats {
	keys := 0
	for _, sh := range st.shards {
		keys += sh.Len()
	}
	return Stats{Hits: st.hits.Load(), Misses: st.misses.Load(), Keys: keys}
}

// RunJanitor purges expired keys every interval until ctx is cancelled.
// It locks one shard at a time so it never stalls the whole store.
func (st *Store) RunJanitor(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, sh := range st.shards {
				sh.PurgeExpired()
			}
		}
	}
}
