package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

func crawl(url string, wg *sync.WaitGroup, limiter *TokenBucket) {
	defer wg.Done()
	limiter.Wait()

	fmt.Printf("Fetching: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	htmlString := string(body)
	re := regexp.MustCompile(`href="([^"]+)"`)
	matches := re.FindAllStringSubmatch(htmlString, -1)
	fmt.Printf("Finished %s. Found %d links.\n", url, len(matches))
}

func main() {
	fmt.Println("=== Go Web Crawler (Day 6 - Rate Limiter) ===")
	
	urls := []string{
		"https://example.com",
		"https://go.dev",
		"https://www.google.com",
		"https://github.com",
	}

	bf := NewBloomFilter(1000)
	limiter := NewTokenBucket(2, 1*time.Second)
	defer limiter.Stop()
	
	var wg sync.WaitGroup

	for _, url := range urls {
		if bf.Exists(url) {
			continue
		}
		bf.Add(url)
		wg.Add(1)
		go crawl(url, &wg, limiter)
	}

	wg.Wait()
	fmt.Println("All crawls completed.")
}
