package retrieval

import (
	"container/heap"
	"fmt"
	"math"
)

// Chunk represents a unit of context passed to the LLM.
// It contains the text content along with its source and optional score.
type Chunk struct {
	Text   string
	Source string
	Score  float64

	// Optional data for advanced retrieval
	Embedding []float64
}

// JoinChunks formats returns a formatted string of Chunks printing or for LLM prompting.
// It prepends ranking and score as metadata.
func JoinChunks(chunks ...Chunk) string {
	var out string
	for i, c := range chunks {
		out += fmt.Sprintf(
			"\n[%d] (score: %.3f)\n%s\n\n---\n",
			i+1,
			c.Score,
			c.Text,
		)
	}
	return out
}

// TopK returns the k highest-scoring chunks from the input slice.
//
// It uses a min-heap to efficiently keep track of the top k elements
// in O(n log k) time, avoiding a full sort of all chunks.
//
// Chunks are expected to have their Score field already populated.
// The returned slice is sorted in descending order of Score.
//
// If k >= len(chunks), all chunks are returned sorted by Score.
func TopK(chunks []Chunk, k int) []Chunk {
	h := &ChunkHeap{}
	heap.Init(h)

	for _, c := range chunks {
		if h.Len() < k {
			heap.Push(h, c)
			continue
		}
		if c.Score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, c)
		}
	}

	results := make([]Chunk, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(Chunk)
	}
	return results
}

// simFunc defines a similarity function between two chunks.
//
// It is used by ranking algorithms (e.g. MMR) to estimate how similar
// two chunks are, typically returning a value in the range [0, 1],
// where 1 means identical and 0 means completely dissimilar.
//
// The actual implementation depends on the use case:
//   - cosine similarity for embeddings
//   - lexical similarity for text
//   - or any custom metric
type simFunc func(a, b Chunk) float64

// MMR (Max Marginal Relevance) selects up to k chunks by balancing:
//
//  1. Relevance (precomputed in Chunk.Score)
//  2. Diversity (penalizing similarity to already selected chunks)
//
// At each step, it selects the chunk maximizing:
//
//	score = λ * relevance + (1 - λ) * (1 - max_similarity_to_selected)
//
// where:
//   - λ ∈ [0,1] controls the tradeoff between relevance and diversity
//   - simFunc provides pairwise similarity between chunks
//
// Notes:
//   - This is an equivalent, "positive-only" formulation of the classic MMR:
//     λ*relevance - (1-λ)*similarity
//     The two differ by a constant and yield identical rankings, but this
//     version is easier to reason about (relevance + diversity).
//   - On the first iteration, max_similarity_to_selected = 0, so selection
//     reduces to choosing the highest-relevance chunk.
func MMR(chunks []Chunk, k int, lambda float64, simF simFunc) []Chunk {
	selected := make([]Chunk, 0, k)
	candidates := make([]Chunk, len(chunks))
	copy(candidates, chunks)
	diversityWeight := 1 - lambda

	for len(selected) < k && len(candidates) > 0 {
		var bestIdx int
		bestScore := math.Inf(-1)

		for i, cand := range candidates {
			// Determine how similar this candidate to other picked items.
			var maxSimToSelected float64
			for _, sel := range selected {
				sim := simF(cand, sel)
				if sim > maxSimToSelected {
					maxSimToSelected = sim
				}
			}

			// Determine score based on relevace but penalise if it's too similar to what we already have.
			score := lambda*cand.Score + diversityWeight*(1-maxSimToSelected)

			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		selected = append(selected, candidates[bestIdx])
		// remove selected candidate (use swap&pop for efficiency over Delete)
		candidates[bestIdx] = candidates[len(candidates)-1]
		candidates = candidates[:len(candidates)-1]
	}

	return selected
}
