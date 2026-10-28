# go-web-crawler

A concurrent web crawler in Go that can run as several cooperating worker processes. URLs are shared through RabbitMQ, visited pages are tracked with a Bloom filter stored in Redis, and each host is rate limited with a token bucket.

## How it works

    seeds -> RabbitMQ (url queue) -> worker goroutines -> fetch + parse links
                  ^                                              |
                  |                  Bloom filter (Redis bitmap) |
                  +---- unseen links, depth + 1 <----------------+

- **BFS traversal.** The queue is FIFO and every task carries its depth, so pages are visited level by level up to `-depth`.
- **Bloom filter.** Visited URLs are tracked in a packed bit array with k hashes derived from one FNV-64 hash (double hashing). Size and k come from the expected URL count and target false-positive rate. At 1M URLs and 1% that is about 1.2 MB. In Redis it is a bitmap updated with `SETBIT` inside `MULTI/EXEC`, so all workers share one visited set. A false positive means a page is skipped, a page is never crawled twice by accident.
- **RabbitMQ.** Durable queue, persistent messages, manual acks. If a worker dies mid-fetch the message is redelivered. `-workers` is also used as the prefetch count.
- **Token bucket.** One bucket per host, refilled from elapsed time (no timer goroutine). Allows a small burst and then a steady rate, so a site is never flooded.
- **Fallbacks.** If Redis or RabbitMQ can't be reached, the crawler falls back to an in-memory Bloom filter and queue, so it still runs on a laptop with nothing installed. `-local` forces this.

## Run it

Start the dependencies:

    docker compose up -d

Crawl:

    go run . -seed https://go.dev -depth 2 -workers 8 -rate 2

To scale out, start more workers in other terminals. They only consume from the queue:

    go run . -workers 8

Without Docker:

    go run . -local -seed https://go.dev

Flags:

| flag | default | meaning |
|---|---|---|
| `-seed` | | comma-separated start URLs, empty means worker only |
| `-depth` | 2 | hops from a seed to follow |
| `-workers` | 8 | concurrent fetchers per process |
| `-rate` / `-burst` | 2 / 2 | per-host requests per second / bucket size |
| `-same-host` | true | stay on the seed's host |
| `-idle` | 5s | a worker exits after this long with no work |
| `-bloom-n` / `-bloom-p` | 1000000 / 0.01 | expected URLs / false-positive rate |
| `-rabbit`, `-redis` | localhost | broker addresses |
| `-local` | false | in-memory only |

## Tests

    go test -race ./...

Covers the Bloom filter (no false negatives, false-positive rate), the token bucket, the queue ordering, link extraction, and an end-to-end crawl against a local test server that has a cycle, an off-site link and a depth cutoff.

## Limitations

- The rate limit is per process, so N worker processes can send N times the configured rate to one host.
- It doesn't read robots.txt yet.
- Termination is idle-based: a worker exits after `-idle` with nothing to do, since a distributed queue has no natural "finished" signal.
- Delivery is at-least-once, so a page can occasionally be fetched twice after a worker crash.
