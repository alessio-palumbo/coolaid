package web

import (
	"ai-cli/internal/retrieval"
	"cmp"
	"slices"
)

// selectTop selects up to max chunks prioritizing both relevance and source diversity.
// It first takes the highest-scoring chunk from each source, then fills remaining
// slots with the next best chunks across all sources, sorted by score.
func selectTop(query string, chunks []retrieval.Chunk, max int) []retrieval.Chunk {
	grouped := rankChunks(query, chunks)

	var result []retrieval.Chunk

	// 1️⃣ take best per source
	for _, list := range grouped {
		if len(list) > 0 {
			result = append(result, list[0])
		}
	}

	// 2️⃣ collect remaining
	var rest []retrieval.Chunk
	for _, list := range grouped {
		if len(list) > 1 {
			rest = append(rest, list[1:]...)
		}
	}

	// sort remaining globally
	slices.SortFunc(rest, func(i, j retrieval.Chunk) int {
		return cmp.Compare(j.Score, i.Score)
	})

	// 3️⃣ fill up to max
	for _, r := range rest {
		if len(result) >= max {
			break
		}
		result = append(result, r)
	}

	// final sort
	slices.SortFunc(result, func(i, j retrieval.Chunk) int {
		return cmp.Compare(j.Score, i.Score)
	})

	// trim if needed
	if len(result) > max {
		result = result[:max]
	}

	return result
}

// rankChunks groups chunks by their source and assigns a relevance score
// to each chunk. Chunks within each source are sorted in descending order
// of score, so the most relevant chunk per source comes first.
func rankChunks(query string, chunks []retrieval.Chunk) map[string][]retrieval.Chunk {
	bm25 := retrieval.NewBM25(chunks)
	bm25.ScoreAndNormalize(query, chunks)

	grouped := make(map[string][]retrieval.Chunk)
	for _, c := range chunks {
		grouped[c.Source] = append(grouped[c.Source], c)
	}

	for k := range grouped {
		slices.SortFunc(grouped[k], func(i, j retrieval.Chunk) int {
			return cmp.Compare(j.Score, i.Score)
		})
	}
	return grouped
}
