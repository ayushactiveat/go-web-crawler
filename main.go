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
	fmt.Println("=== Go Web Crawler (RabbitMQ Distributed) ===")
	
	qm, err := NewQueueManager("amqp://guest:guest@localhost:5672/")
	if err != nil {
		fmt.Println("Warning: RabbitMQ not running. Falling back to local mode.")
	} else {
		defer qm.Close()
		fmt.Println("Connected to RabbitMQ.")
	}

	urls := []string{
		"https://example.com",
		"https://go.dev",
		"https://www.google.com",
	}

	bf := NewBloomFilter(1000)
	limiter := NewTokenBucket(2, 1*time.Second)
	defer limiter.Stop()
	
	var wg sync.WaitGroup

	if qm != nil {
		msgs, _ := qm.Consume()
		go func() {
			for d := range msgs {
				wg.Add(1)
				go crawl(string(d.Body), &wg, limiter)
			}
		}()
	}

	for _, url := range urls {
		if bf.Exists(url) {
			continue
		}
		bf.Add(url)
		if qm != nil {
			qm.Publish(url)
		} else {
			wg.Add(1)
			go crawl(url, &wg, limiter)
		}
	}

	wg.Wait()
	fmt.Println("All crawls completed.")
}
