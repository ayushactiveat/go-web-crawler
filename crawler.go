package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Workers  int
	MaxDepth int
	SameHost bool
	Idle     time.Duration // a worker exits after this long without a message
}

type Crawler struct {
	cfg     Config
	queue   Queue
	bloom   *Bloom
	limiter *HostLimiter
	client  *http.Client

	fetched, failed, enqueued atomic.Int64
}

func NewCrawler(cfg Config, q Queue, b *Bloom, l *HostLimiter) *Crawler {
	return &Crawler{
		cfg:     cfg,
		queue:   q,
		bloom:   b,
		limiter: l,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Seed puts the starting URLs on the queue at depth 0.
func (c *Crawler) Seed(ctx context.Context, seeds []string) {
	for _, s := range seeds {
		u := normalize(&url.URL{}, s)
		if u == "" {
			log.Printf("skipping bad seed %q", s)
			continue
		}
		parsed, _ := url.Parse(u)
		c.enqueue(ctx, Task{URL: u, Depth: 0, Origin: parsed.Host})
	}
}

// enqueue publishes a URL unless the Bloom filter says we've already queued it.
func (c *Crawler) enqueue(ctx context.Context, t Task) {
	seen, err := c.bloom.TestAndAdd(ctx, t.URL)
	if err != nil {
		log.Printf("bloom check failed for %s: %v", t.URL, err)
		return
	}
	if seen {
		return
	}
	if err := c.queue.Publish(ctx, t); err != nil {
		log.Printf("publish failed for %s: %v", t.URL, err)
		return
	}
	c.enqueued.Add(1)
}

// Run starts the worker pool and returns once every worker has gone idle or
// ctx is cancelled. Because the queue is FIFO and tasks carry their depth,
// pages are visited in breadth-first order.
func (c *Crawler) Run(ctx context.Context) error {
	msgs, err := c.queue.Consume(ctx)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(ctx, msgs)
		}()
	}
	wg.Wait()
	return nil
}

func (c *Crawler) worker(ctx context.Context, msgs <-chan Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.cfg.Idle):
			return
		case m, ok := <-msgs:
			if !ok {
				return
			}
			c.process(ctx, m.Task)
			// if we're shutting down, leave it unacked so it gets redelivered
			if ctx.Err() == nil {
				m.Ack()
			}
		}
	}
}

func (c *Crawler) process(ctx context.Context, t Task) {
	u, err := url.Parse(t.URL)
	if err != nil {
		return
	}
	if err := c.limiter.Wait(ctx, u.Host); err != nil {
		return
	}

	links, err := fetch(ctx, c.client, t.URL)
	if err != nil {
		c.failed.Add(1)
		log.Printf("fail  depth=%d %s: %v", t.Depth, t.URL, err)
		return
	}
	c.fetched.Add(1)
	log.Printf("ok    depth=%d %s (%d links)", t.Depth, t.URL, len(links))

	if t.Depth >= c.cfg.MaxDepth {
		return
	}
	for _, l := range links {
		if c.cfg.SameHost {
			lu, err := url.Parse(l)
			if err != nil || lu.Host != t.Origin {
				continue
			}
		}
		c.enqueue(ctx, Task{URL: l, Depth: t.Depth + 1, Origin: t.Origin})
	}
}

func (c *Crawler) Stats() (fetched, failed, enqueued int64) {
	return c.fetched.Load(), c.failed.Load(), c.enqueued.Load()
}
