package web

import (
	"coolaid/internal/retrieval"
	"strings"
)

// mmrLambda controls the tradeoff between relevance (1) and diversity (0).
const mmrLambda = 0.7

// mmrOversampleFactor is the k-multiplier used to search for candidates.
const mmrOversampleFactor = 4

// selectTop returns up to max chunks using MMR for balancing
// relevance (Chunk.Score) and diversity (via similarity function).
func selectTop(query string, chunks []retrieval.Chunk, max int) []retrieval.Chunk {
	if len(chunks) == 0 || max <= 0 {
		return nil
	}

	bm25 := retrieval.NewBM25(chunks)
	bm25.ScoreAndNormalize(query, chunks)

	chunks = retrieval.TopK(chunks, max*mmrOversampleFactor)

	// apply MMR directly over scored chunks
	// lambda controls relevance vs diversity tradeoff
	return retrieval.MMR(chunks, max, mmrLambda, idfOverlapSim(bm25))
}

// idfOverlapSim computes similarity between two chunks based on
// IDF-weighted token overlap.
//
// Instead of treating all tokens equally (as in raw token overlap),
// this function weights each token by its inverse document frequency
// (IDF), giving more importance to rare and informative terms.
//
// Similarity is computed as:
//
//	sim(a, b) = sum(IDF(tokens shared by a and b)) / sum(IDF(tokens in b))
//
// The result is in [0,1]:
//   - 1.0 → all informative tokens in b appear in a
//   - 0.0 → no overlap in informative tokens
//
// This makes it more robust than raw token overlap for MMR diversity,
// especially for web text where stopwords dominate.
func idfOverlapSim(bm *retrieval.BM25) retrieval.SimFunc {
	return func(a, b retrieval.Chunk) float64 {
		aTokens := strings.Fields(a.Text)
		bTokens := strings.Fields(b.Text)

		aSet := make(map[string]struct{})
		for _, t := range aTokens {
			aSet[t] = struct{}{}
		}

		var score float64
		var norm float64

		for _, t := range bTokens {
			idf := bm.IDF(t)
			norm += idf

			if _, ok := aSet[t]; ok {
				score += idf
			}
		}

		if norm == 0 {
			return 0
		}

		return score / norm
	}
}
