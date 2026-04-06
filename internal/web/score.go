package web

import (
	"ai-cli/internal/retrieval"
	"cmp"
	"slices"
	"strings"
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
	grouped := make(map[string][]retrieval.Chunk)
	for _, c := range chunks {
		c.Score = scoreChunk(query, c)
		grouped[c.Source] = append(grouped[c.Source], c)
	}

	for k := range grouped {
		slices.SortFunc(grouped[k], func(i, j retrieval.Chunk) int {
			return cmp.Compare(j.Score, i.Score)
		})
	}
	return grouped
}

// scoreChunk assigns a relevance score in [0.0, 1.0] to a chunk based on
// simple keyword matching against the query. The score represents the
// fraction of query terms present in the chunk text.
func scoreChunk(query string, c retrieval.Chunk) float64 {
	qWords := strings.Fields(strings.ToLower(query))
	if len(qWords) == 0 {
		return 0
	}

	text := strings.ToLower(c.Text)
	matches := 0
	for _, w := range qWords {
		if strings.Contains(text, w) {
			matches++
		}
	}
	return float64(matches) / float64(len(qWords))
	// favors richer chunks slightly
	// return float64(matches) / float64(len(qWords)) * math.Min(1, float64(len(c.Text))/2000)
}
