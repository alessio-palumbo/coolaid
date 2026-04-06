package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Result represents a single search result returned by a Searcher.
type Result struct {
	Title string
	URL   string
}

type Chunk struct {
	Text string
	URL  string
}

// Searcher defines a component capable of querying a search engine
// and returning a list of relevant results.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// DuckDuckGo implements the Searcher interface using the DuckDuckGo
// HTML endpoint, suitable for lightweight scraping without an API key.
type DuckDuckGo struct {
	Client *http.Client
}

func NewDuckDuckGo(client *http.Client) *DuckDuckGo {
	return &DuckDuckGo{Client: client}
}

// Search performs a query against DuckDuckGo and returns the top results.
// It parses the HTML response and extracts titles and cleaned URLs.
func (d *DuckDuckGo) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	q := url.QueryEscape(query)
	endpoint := "https://html.duckduckgo.com/html/?q=" + q

	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []Result

	doc.Find(".result").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(results) >= limit {
			return false
		}

		link := s.Find(".result__a")
		href, _ := link.Attr("href")
		href = d.cleanURL(href)
		title := strings.TrimSpace(link.Text())

		if href != "" {
			results = append(results, Result{
				Title: title,
				URL:   href,
			})
		}

		return true
	})

	return results, nil
}

// cleanURL extracts and normalizes a URL (DuckDuckGo wraps links in redirect URLs).
func (d *DuckDuckGo) cleanURL(raw string) string {
	if raw == "" {
		return ""
	}

	// scheme-less
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if q := u.Query().Get("uddg"); q != "" {
		if decoded, err := url.QueryUnescape(q); err == nil {
			return decoded
		}
	}

	return raw
}
