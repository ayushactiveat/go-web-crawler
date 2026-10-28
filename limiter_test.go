package main

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucketBurstThenThrottle(t *testing.T) {
	tb := NewTokenBucket(2, 10) // 2 token burst, then 1 every 100ms
	ctx := context.Background()

	start := time.Now()
	tb.Wait(ctx)
	tb.Wait(ctx)
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("the burst should not have blocked")
	}
	tb.Wait(ctx)
	if time.Since(start) < 80*time.Millisecond {
		t.Fatal("third request should have waited for a refill")
	}
}

func TestTokenBucketRespectsContext(t *testing.T) {
	tb := NewTokenBucket(1, 0.1)
	tb.Wait(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := tb.Wait(ctx); err == nil {
		t.Fatal("expected an error once the context expired")
	}
}
