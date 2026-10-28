package main

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// BitStore is where a Bloom filter keeps its bits.
type BitStore interface {
	// TestAndSet sets every bit in idx and reports whether all of them
	// were already set beforehand.
	TestAndSet(ctx context.Context, idx []uint64) (bool, error)
}

// MemoryStore is a plain bit array, 64 bits per word.
type MemoryStore struct {
	mu   sync.Mutex
	bits []uint64
}

func NewMemoryStore(m uint64) *MemoryStore {
	return &MemoryStore{bits: make([]uint64, (m+63)/64)}
}

func (s *MemoryStore) TestAndSet(_ context.Context, idx []uint64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	present := true
	for _, b := range idx {
		word, mask := b/64, uint64(1)<<(b%64)
		if s.bits[word]&mask == 0 {
			present = false
			s.bits[word] |= mask
		}
	}
	return present, nil
}

// RedisStore keeps the filter in a Redis bitmap so every worker process
// sees the same set of visited URLs.
type RedisStore struct {
	rdb *redis.Client
	key string
}

func NewRedisStore(ctx context.Context, addr, key string) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		rdb.Close()
		return nil, err
	}
	return &RedisStore{rdb: rdb, key: key}, nil
}

func (s *RedisStore) TestAndSet(ctx context.Context, idx []uint64) (bool, error) {
	// MULTI/EXEC so another worker can't interleave between our SETBITs.
	pipe := s.rdb.TxPipeline()
	cmds := make([]*redis.IntCmd, len(idx))
	for i, b := range idx {
		cmds[i] = pipe.SetBit(ctx, s.key, int64(b), 1)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	// SETBIT returns the old value of each bit.
	for _, c := range cmds {
		if c.Val() == 0 {
			return false, nil
		}
	}
	return true, nil
}

func (s *RedisStore) Close() error { return s.rdb.Close() }
