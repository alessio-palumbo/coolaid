package web

import (
	"coolaid/internal/retrieval"
	"testing"
)

func TestSelectTop_DiversityAndSorting(t *testing.T) {
	chunks := []retrieval.Chunk{
		{Text: "A1", Source: "s1", Score: 0.9},
		{Text: "A2", Source: "s1", Score: 0.8},
		{Text: "B1", Source: "s2", Score: 0.95},
		{Text: "B2", Source: "s2", Score: 0.7},
		{Text: "C1", Source: "s3", Score: 0.85},
	}

	out := selectTop("query", chunks, 3)

	if len(out) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(out))
	}

	// check ordering (descending score)
	for i := 1; i < len(out); i++ {
		if out[i-1].Score < out[i].Score {
			t.Errorf("chunks not sorted by score")
		}
	}

	// check diversity (at least 2 different sources)
	sources := make(map[string]bool)
	for _, c := range out {
		sources[c.Source] = true
	}

	if len(sources) < 2 {
		t.Errorf("expected diversity across sources")
	}
}
