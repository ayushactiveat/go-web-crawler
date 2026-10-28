package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	seeds := flag.String("seed", "", "comma-separated seed URLs (leave empty to run as a worker only)")
	workers := flag.Int("workers", 8, "concurrent fetchers in this process")
	maxDepth := flag.Int("depth", 2, "how many hops from a seed to follow")
	rate := flag.Float64("rate", 2, "requests per second allowed per host")
	burst := flag.Int("burst", 2, "token bucket capacity per host")
	sameHost := flag.Bool("same-host", true, "stay on the seed's host")
	idle := flag.Duration("idle", 5*time.Second, "exit after this long with nothing to do")
	rabbitURL := flag.String("rabbit", "amqp://guest:guest@localhost:5672/", "RabbitMQ URL")
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	bloomN := flag.Uint64("bloom-n", 1_000_000, "expected number of distinct URLs")
	bloomP := flag.Float64("bloom-p", 0.01, "target Bloom filter false-positive rate")
	local := flag.Bool("local", false, "skip RabbitMQ and Redis and run fully in memory")
	flag.Parse()

	log.SetFlags(log.Ltime)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m, k := BloomParams(*bloomN, *bloomP)
	var store BitStore = NewMemoryStore(m)
	var q Queue = NewMemQueue()

	if !*local {
		if rs, err := NewRedisStore(ctx, *redisAddr, "crawler:bloom"); err != nil {
			log.Printf("redis unavailable (%v), using an in-memory bloom filter", err)
		} else {
			defer rs.Close()
			store = rs
			log.Printf("bloom filter stored in redis at %s", *redisAddr)
		}
		if rq, err := NewRabbitQueue(*rabbitURL, *workers); err != nil {
			log.Printf("rabbitmq unavailable (%v), using an in-memory queue", err)
		} else {
			q = rq
			log.Printf("connected to rabbitmq")
		}
	}
	defer q.Close()

	if _, inMemory := q.(*MemQueue); inMemory && *seeds == "" {
		log.Printf("nothing to do: in-memory queue and no -seed given")
		return
	}

	bloom := NewBloom(store, m, k)
	limiter := NewHostLimiter(*burst, *rate)
	c := NewCrawler(Config{
		Workers:  *workers,
		MaxDepth: *maxDepth,
		SameHost: *sameHost,
		Idle:     *idle,
	}, q, bloom, limiter)

	if *seeds != "" {
		var list []string
		for _, s := range strings.Split(*seeds, ",") {
			if s = strings.TrimSpace(s); s != "" {
				list = append(list, s)
			}
		}
		c.Seed(ctx, list)
	}

	start := time.Now()
	if err := c.Run(ctx); err != nil {
		log.Fatal(err)
	}
	fetched, failed, enqueued := c.Stats()
	log.Printf("done in %s: %d fetched, %d failed, %d queued",
		time.Since(start).Round(time.Millisecond), fetched, failed, enqueued)
}
