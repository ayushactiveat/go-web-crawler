package main

import (
	"context"
	"fmt"
	"testing"
)

func TestBloomNoFalseNegatives(t *testing.T) {
	m, k := BloomParams(1000, 0.01)
	b := NewBloom(NewMemoryStore(m), m, k)
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		b.TestAndAdd(ctx, fmt.Sprintf("https://example.com/page/%d", i))
	}
	for i := 0; i < 1000; i++ {
		seen, err := b.TestAndAdd(ctx, fmt.Sprintf("https://example.com/page/%d", i))
		if err != nil || !seen {
			t.Fatalf("page %d should be reported as seen", i)
		}
	}
}

func TestBloomFalsePositiveRate(t *testing.T) {
	const n = 5000
	m, k := BloomParams(n, 0.01)
	b := NewBloom(NewMemoryStore(m), m, k)
	ctx := context.Background()

	// every URL here is distinct, so any "seen" answer is a false positive
	fp := 0
	for i := 0; i < n; i++ {
		if seen, _ := b.TestAndAdd(ctx, fmt.Sprintf("https://site.test/item/%d", i)); seen {
			fp++
		}
	}
	if rate := float64(fp) / n; rate > 0.02 {
		t.Fatalf("false positive rate %.4f is well above the 1%% target", rate)
	}
}
