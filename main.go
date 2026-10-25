package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
)

func crawl(url string, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Starting crawl on: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading body from %s: %v\n", url, err)
		return
	}

	htmlString := string(body)
	re := regexp.MustCompile(`href="([^"]+)"`)
	matches := re.FindAllStringSubmatch(htmlString, -1)

	fmt.Printf("Finished %s. Found %d links.\n", url, len(matches))
}

func main() {
	fmt.Println("=== Go Web Crawler (Day 5 - Bloom Filter) ===")

	urls := []string{
		"https://example.com",
		"https://go.dev",
		"https://www.google.com",
		"https://example.com", 
	}

	bf := NewBloomFilter(1000)
	var wg sync.WaitGroup

	for _, url := range urls {
		if bf.Exists(url) {
			fmt.Printf("Skipping (already visited): %s\n", url)
			continue
		}
		
		bf.Add(url)
		wg.Add(1)
		go crawl(url, &wg)
	}

	wg.Wait()

	fmt.Println("All crawls completed.")
}
