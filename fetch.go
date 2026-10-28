package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const (
	userAgent = "go-web-crawler/0.1 (+https://github.com/ayushactiveat/go-web-crawler)"
	maxBody   = 2 << 20 // don't read more than 2 MiB of any page
)

// fetch downloads a page and returns the links found on it.
func fetch(ctx context.Context, client *http.Client, rawURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		return nil, nil
	}
	// resolve relative links against the final URL, after any redirects
	return extractLinks(io.LimitReader(resp.Body, maxBody), resp.Request.URL)
}

// extractLinks parses HTML and returns the unique, absolute http(s) links in
// <a href> tags.
func extractLinks(r io.Reader, base *url.URL) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				if u := normalize(base, a.Val); u != "" && !seen[u] {
					seen[u] = true
					links = append(links, u)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links, nil
}

// normalize resolves href against base and drops anything that isn't a plain
// http(s) page. Fragments are stripped so /a and /a#top count as one URL.
func normalize(base *url.URL, href string) string {
	u, err := base.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
