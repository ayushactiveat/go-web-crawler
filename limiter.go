package main

import "time"

type TokenBucket struct {
	tokens chan struct{}
	stop   chan struct{}
}

func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
	tb := &TokenBucket{tokens: make(chan struct{}, capacity), stop: make(chan struct{})}
	for i := 0; i < capacity; i++ {
		tb.tokens <- struct{}{}
	}
	go func() {
		ticker := time.NewTicker(refillRate)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case tb.tokens <- struct{}{}:
				default:
				}
			case <-tb.stop:
				return
			}
		}
	}()
	return tb
}

func (tb *TokenBucket) Wait() { <-tb.tokens }
func (tb *TokenBucket) Stop() { close(tb.stop) }
