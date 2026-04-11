package web

import (
	"coolaid/internal/retrieval"
	"testing"
)

func TestSelectTop(t *testing.T) {
	tests := []struct {
		name  string
		query string
		input []retrieval.Chunk
		max   int
	}{
		{
			name:  "basic selection",
			query: "golang concurrency",
			max:   3,
			input: []retrieval.Chunk{
				{Text: "golang concurrency patterns", Source: "a"},
				{Text: "goroutines and channels", Source: "a"},
				{Text: "python threading basics", Source: "b"},
				{Text: "golang channels tutorial", Source: "c"},
				{Text: "java concurrency guide", Source: "d"},
			},
		},
		{
			name:  "empty input",
			query: "test",
			max:   5,
			input: nil,
		},
		{
			name:  "max zero",
			query: "test",
			max:   0,
			input: []retrieval.Chunk{
				{Text: "something", Source: "a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := selectTop(tt.query, tt.input, tt.max)

			if tt.input == nil || tt.max <= 0 {
				if out != nil {
					t.Fatalf("expected nil, got %v", out)
				}
				return
			}

			if len(out) > tt.max {
				t.Fatalf("expected at most %d results, got %d", tt.max, len(out))
			}

			// basic sanity: no empty chunks
			for _, c := range out {
				if c.Text == "" {
					t.Fatalf("empty chunk returned")
				}
			}

			// optional: check some diversity (not all same source)
			if len(out) > 1 {
				first := out[0].Source
				allSame := true
				for _, c := range out {
					if c.Source != first {
						allSame = false
						break
					}
				}
				if allSame {
					t.Log("warning: low diversity in results (may be acceptable)")
				}
			}
		})
	}
}
