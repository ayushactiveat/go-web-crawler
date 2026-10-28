package main

import (
	"context"
	"testing"
	"time"
)

func TestMemQueueIsFIFO(t *testing.T) {
	q := NewMemQueue()
	defer q.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, u := range []string{"a", "b", "c"} {
		q.Publish(ctx, Task{URL: u})
	}
	msgs, _ := q.Consume(ctx)
	for _, want := range []string{"a", "b", "c"} {
		select {
		case m := <-msgs:
			if m.URL != want {
				t.Fatalf("got %q, want %q", m.URL, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}
