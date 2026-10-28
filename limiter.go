package main

import (
	"context"
	"sync"
	"time"
)

// TokenBucket allows bursts up to capacity and refills at a steady rate.
// Tokens are computed from elapsed time on each call, so there is no
// background goroutine to start or stop.
type TokenBucket struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	rate     float64 // tokens per second
	last     time.Time
}

func NewTokenBucket(capacity int, perSecond float64) *TokenBucket {
	return &TokenBucket{
		capacity: float64(capacity),
		tokens:   float64(capacity),
		rate:     perSecond,
		last:     time.Now(),
	}
}

// Wait blocks until a token is available or ctx is cancelled.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		tb.mu.Lock()
		now := time.Now()
		tb.tokens = min(tb.capacity, tb.tokens+now.Sub(tb.last).Seconds()*tb.rate)
		tb.last = now
		if tb.tokens >= 1 {
			tb.tokens--
			tb.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - tb.tokens) / tb.rate * float64(time.Second))
		tb.mu.Unlock()

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// HostLimiter keeps one bucket per host so a slow site doesn't hold up the
// others, and no single site gets hammered.
type HostLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*TokenBucket
	capacity int
	rate     float64
}

func NewHostLimiter(capacity int, perSecond float64) *HostLimiter {
	return &HostLimiter{buckets: map[string]*TokenBucket{}, capacity: capacity, rate: perSecond}
}

func (h *HostLimiter) Wait(ctx context.Context, host string) error {
	h.mu.Lock()
	b, ok := h.buckets[host]
	if !ok {
		b = NewTokenBucket(h.capacity, h.rate)
		h.buckets[host] = b
	}
	h.mu.Unlock()
	return b.Wait(ctx)
}
