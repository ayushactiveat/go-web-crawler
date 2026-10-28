package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExtractLinks(t *testing.T) {
	base, _ := url.Parse("https://example.com/docs/")
	src := `<a href="intro">a</a> <a href="/x#top">b</a> <a href="mailto:me@example.com">c</a>
		<a href="intro">dup</a> <a href="https://go.dev">d</a>`

	got, err := extractLinks(strings.NewReader(src), base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://example.com/docs/intro", "https://example.com/x", "https://go.dev/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCrawlerBFS(t *testing.T) {
	page := func(links ...string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			for _, l := range links {
				fmt.Fprintf(w, `<a href="%s">link</a>`, l)
			}
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", page("/a", "/b", "https://elsewhere.invalid/x"))
	mux.HandleFunc("GET /a", page("/c", "/b#frag"))
	mux.HandleFunc("GET /b", page("/a")) // cycle back to /a
	mux.HandleFunc("GET /c", page("/d"))
	mux.HandleFunc("GET /d", page())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m, k := BloomParams(1000, 0.01)
	c := NewCrawler(
		Config{Workers: 4, MaxDepth: 2, SameHost: true, Idle: 300 * time.Millisecond},
		NewMemQueue(), NewBloom(NewMemoryStore(m), m, k), NewHostLimiter(100, 1000),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Seed(ctx, []string{srv.URL})
	if err := c.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// /, /a, /b, /c: /d is a hop too far, the cycle and the other host are skipped
	fetched, failed, _ := c.Stats()
	if fetched != 4 || failed != 0 {
		t.Fatalf("fetched=%d failed=%d, want 4 and 0", fetched, failed)
	}
}
