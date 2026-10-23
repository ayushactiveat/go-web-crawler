package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
)

func main() {
	fmt.Println("=== Go Web Crawler (Day 3) ===")

	targetURL := "https://example.com"
	fmt.Println("Fetching:", targetURL)

	resp, err := http.Get(targetURL)
	if err != nil {
		fmt.Println("Error fetching URL:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading body:", err)
		return
	}

	htmlString := string(body)

	// 1. Compile the Regex to find href="..."
	// This pattern looks for href=", followed by any characters that aren't a quote, followed by a quote.
	re := regexp.MustCompile(`href="([^"]+)"`)

	// 2. Find all matches
	matches := re.FindAllStringSubmatch(htmlString, -1)

	fmt.Printf("\nFound %d links:\n", len(matches))

	// 3. Loop through the matches and print them
	// matches[i][1] contains the actual URL (the capture group)
	for i, match := range matches {
		fmt.Printf("%d: %s\n", i+1, match[1])
	}
}
