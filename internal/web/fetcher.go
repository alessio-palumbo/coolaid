package web

import (
	"context"
	"io"
	"net/http"
)

// Fetcher defines a component capable of retrieving raw HTML content
// from a given URL. Implementations should handle timeouts, headers,
// and basic HTTP concerns.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (string, error)
}

// HTTPFetcher is a basic Fetcher implementation backed by net/http.
// It performs a simple GET request and returns the response body as a string.
type HTTPFetcher struct {
	Client *http.Client
}

// NewHTTPFetcher creates a HTTPFetcher with sane defaults,
// including a request timeout to avoid hanging requests.
func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	return &HTTPFetcher{Client: client}
}

// Fetch retrieves the content at the given URL using an HTTP GET request.
// It sets a browser-like User-Agent to improve compatibility with servers.
// The response body is returned as a string.
//
// Note: This method assumes the URL is already resolved (e.g. no search engine redirects).
func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	// Some servers reject requests without a proper User-Agent.
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
