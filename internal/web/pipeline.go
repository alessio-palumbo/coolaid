package web

import (
	"coolaid/internal/retrieval"
	"context"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	fetchLimiterPeriod = 500 * time.Millisecond
	fetchJitterMax     = 200
	maxConcurrentJobs  = 5

	defaultHTTPTimeout = 10 * time.Second
	minChunkSize       = 300
)

// Pipeline orchestrates web-based retrieval by combining search,
// fetching, extraction, and chunking into a single workflow.
type Pipeline struct {
	Searcher  Searcher
	Fetcher   Fetcher
	Extractor Extractor
	Chunker   func(string) []string
	Limit     int
}

type Option func(*Pipeline)

func WithSearcher(s Searcher) Option {
	return func(p *Pipeline) { p.Searcher = s }
}

func WithChunker(c func(string) []string) Option {
	return func(p *Pipeline) { p.Chunker = c }
}

func NewPipeline(limit int, opts ...Option) *Pipeline {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	p := &Pipeline{
		Searcher:  NewDuckDuckGo(client),
		Fetcher:   NewHTTPFetcher(client),
		Extractor: NewSimpleExtractor(),
		Chunker:   NewTextChunker().Chunk,
		Limit:     limit,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Run executes the full retrieval pipeline for a query.
// It searches for relevant pages, fetches and extracts their content,
// then splits the result into chunks for downstream use.
func (p *Pipeline) Run(ctx context.Context, query string) ([]retrieval.Chunk, error) {
	results, err := p.Searcher.Search(ctx, query, p.Limit)
	if err != nil {
		return nil, err
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		chunks []retrieval.Chunk
	)

	jobs := make(chan struct{}, maxConcurrentJobs)
	limiter := time.NewTicker(fetchLimiterPeriod)

	for _, r := range results {
		wg.Add(1)

		go func(url string) {
			defer wg.Done()
			jobs <- struct{}{}
			defer func() { <-jobs }()

			<-limiter.C
			// jitter
			time.Sleep(time.Duration(rand.Intn(fetchJitterMax)) * time.Millisecond)

			html, err := p.Fetcher.Fetch(ctx, url)
			if err != nil {
				return
			}

			text, err := p.Extractor.Extract(html)
			if err != nil {
				return
			}

			parts := p.Chunker(text)

			mu.Lock()
			for _, part := range parts {
				if len(part) < minChunkSize {
					continue
				}
				chunks = append(chunks, retrieval.Chunk{
					Text:   part,
					Source: url,
				})
			}
			mu.Unlock()

		}(r.URL)
	}

	wg.Wait()

	return selectTop(query, chunks, p.Limit), nil
}
