package retrieval

import "testing"

func TestBM25Scoring(t *testing.T) {
	chunks := []Chunk{
		{Text: "Go is a programming language"},
		{Text: "Python is also a programming language"},
		{Text: "Bananas are yellow"},
	}

	bm := NewBM25(chunks)
	bm.ScoreAndNormalize("go programming", chunks)

	if chunks[0].Score <= chunks[2].Score {
		t.Errorf("expected Go chunk to score higher than irrelevant chunk")
	}
	if chunks[0].Score < chunks[1].Score {
		t.Errorf("expected Go chunk to rank above Python for query 'go programming'")
	}
}
