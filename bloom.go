package main

import (
	"context"
	"hash/fnv"
	"math"
)

// Bloom is a Bloom filter whose bits live in a BitStore, so the same logic
// works for an in-process bitset or a Redis bitmap shared between workers.
type Bloom struct {
	store BitStore
	m, k  uint64
}

// BloomParams returns the number of bits (m) and hash functions (k) needed to
// hold n items at roughly false-positive rate p.
func BloomParams(n uint64, p float64) (m, k uint64) {
	m = uint64(math.Ceil(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)))
	k = uint64(math.Round(float64(m) / float64(n) * math.Ln2))
	if k < 1 {
		k = 1
	}
	return m, k
}

func NewBloom(store BitStore, m, k uint64) *Bloom {
	return &Bloom{store: store, m: m, k: k}
}

// indexes derives k bit positions from one 64-bit FNV hash using double
// hashing (h1 + i*h2), which avoids running k separate hash functions.
func (b *Bloom) indexes(s string) []uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	sum := h.Sum64()
	h1 := sum & 0xffffffff
	h2 := (sum >> 32) | 1

	idx := make([]uint64, b.k)
	for i := uint64(0); i < b.k; i++ {
		idx[i] = (h1 + i*h2) % b.m
	}
	return idx
}

// TestAndAdd marks s as seen and reports whether it was probably seen before.
// A true result can be a false positive, a false result never is.
func (b *Bloom) TestAndAdd(ctx context.Context, s string) (bool, error) {
	return b.store.TestAndSet(ctx, b.indexes(s))
}
