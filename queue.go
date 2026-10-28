package main

import (
	"context"
	"sync"
)

// Task is one URL to crawl. Depth is the number of hops from the seed and
// Origin is the seed's host, used to stay on one site when asked to.
type Task struct {
	URL    string `json:"url"`
	Depth  int    `json:"depth"`
	Origin string `json:"origin"`
}

// Message is a Task handed to a worker. Ack should be called once the worker
// is finished with it, otherwise a broker-backed queue will redeliver it.
type Message struct {
	Task
	Ack func()
}

type Queue interface {
	Publish(ctx context.Context, t Task) error
	Consume(ctx context.Context) (<-chan Message, error)
	Close() error
}

// MemQueue is an unbounded in-process FIFO, used when RabbitMQ isn't around.
// It has to be unbounded: workers publish new links while they consume, so a
// fixed-size channel can deadlock once every worker is blocked on a full one.
type MemQueue struct {
	mu    sync.Mutex
	items []Task
	sig   chan struct{}
	done  chan struct{}
	once  sync.Once
}

func NewMemQueue() *MemQueue {
	return &MemQueue{sig: make(chan struct{}, 1), done: make(chan struct{})}
}

func (q *MemQueue) Publish(_ context.Context, t Task) error {
	q.mu.Lock()
	q.items = append(q.items, t)
	q.mu.Unlock()
	select {
	case q.sig <- struct{}{}:
	default:
	}
	return nil
}

func (q *MemQueue) pop() (Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return Task{}, false
	}
	t := q.items[0]
	q.items = q.items[1:]
	return t, true
}

func (q *MemQueue) Consume(ctx context.Context) (<-chan Message, error) {
	out := make(chan Message)
	go func() {
		defer close(out)
		for {
			t, ok := q.pop()
			if !ok {
				select {
				case <-q.sig:
					continue
				case <-ctx.Done():
					return
				case <-q.done:
					return
				}
			}
			select {
			case out <- Message{Task: t, Ack: func() {}}:
			case <-ctx.Done():
				return
			case <-q.done:
				return
			}
		}
	}()
	return out, nil
}

func (q *MemQueue) Close() error {
	q.once.Do(func() { close(q.done) })
	return nil
}
