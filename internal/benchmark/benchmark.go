package benchmark

import (
	"context"
	"coolaid/internal/retrieval"
	"fmt"
	"log"
	"strings"
)

var tests = []TestCase{
	{
		Name:  "ask command discovery",
		Query: "how to ask a question about indexed code",
		ExpectedFiles: []string{
			"ask.go",
			"query.go",
			"search.go",
		},
	},
	{
		Name:  "indexing flow",
		Query: "how indexing works",
		ExpectedFiles: []string{
			"index.go",
			"chunk.go",
		},
	},
}

type TestCase struct {
	Name          string
	Query         string
	ExpectedFiles []string
}

type searchOpts struct {
	k      int
	useMMR bool
}

var testSearchOpts = []searchOpts{
	{k: 5, useMMR: false},
	{k: 8, useMMR: false},
	{k: 12, useMMR: true},
}

type Searcher interface {
	SemanticSearch(ctx context.Context, prompt string, k int, useMMR bool) ([]retrieval.Chunk, error)
}

func Run(searcher Searcher) {
	for _, tc := range tests {
		fmt.Println("Test:", tc.Name)
		ctx := context.Background()

		for _, opts := range testSearchOpts {
			chunks, err := searcher.SemanticSearch(ctx, tc.Query, opts.k, opts.useMMR)
			if err != nil {
				log.Fatal(err)
			}
			s := score(chunks, tc.ExpectedFiles)
			fmt.Printf(" (k:%d-mmr:%v): %.2f\n", opts.k, opts.useMMR, s)
			for _, c := range chunks {
				fmt.Println(" -", c.Source)
			}
			fmt.Println()
		}
	}
}

func score(chunks []retrieval.Chunk, expected []string) float64 {
	hits := 0
	for _, e := range expected {
		for _, c := range chunks {
			if strings.Contains(c.Source, e) {
				hits++
				break
			}
		}
	}
	return float64(hits) / float64(len(expected))
}
