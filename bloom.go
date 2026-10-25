package main

import (
	"hash/fnv"
)

type BloomFilter struct {
	bitset []bool
	size   uint
}

func NewBloomFilter(size uint) *BloomFilter {
	return &BloomFilter{
		bitset: make([]bool, size),
		size:   size,
	}
}

func (bf *BloomFilter) hashes(s string) []uint {
	h1 := fnv.New32a()
	h1.Write([]byte(s))
	hash1 := h1.Sum32() % uint32(bf.size)

	h2 := fnv.New32()
	h2.Write([]byte(s))
	hash2 := h2.Sum32() % uint32(bf.size)

	return []uint{uint(hash1), uint(hash2)}
}

func (bf *BloomFilter) Add(url string) {
	for _, h := range bf.hashes(url) {
		bf.bitset[h] = true
	}
}

func (bf *BloomFilter) Exists(url string) bool {
	for _, h := range bf.hashes(url) {
		if !bf.bitset[h] {
			return false
		}
	}
	return true
}
