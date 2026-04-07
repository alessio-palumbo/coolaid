package retrieval

import (
	"math"
	"strings"
	"unicode"
)

type BM25 struct {
	k1        float64
	b         float64
	idf       map[string]float64
	avgDocLen float64
}

// NewBM25 builds corpus statistics (IDF, avg doc length)
func NewBM25(chunks []Chunk) *BM25 {
	N := float64(len(chunks))
	df := make(map[string]float64)
	var totalLen float64

	for _, c := range chunks {
		terms := tokenize(c.Text)
		totalLen += float64(len(terms))

		seen := make(map[string]struct{})
		for _, t := range terms {
			if _, ok := seen[t]; !ok {
				df[t]++
				seen[t] = struct{}{}
			}
		}
	}

	idf := make(map[string]float64)
	for term, freq := range df {
		idf[term] = math.Log(1 + (N-freq+0.5)/(freq+0.5))
	}

	return &BM25{
		k1:        1.5,
		b:         0.75,
		idf:       idf,
		avgDocLen: totalLen / N,
	}
}

// Score computes BM25 relevance score for a chunk given a query.
func (b *BM25) Score(query string, c Chunk) float64 {
	terms := tokenize(c.Text)
	docLen := float64(len(terms))

	tf := make(map[string]float64)
	for _, t := range terms {
		tf[t]++
	}

	var score float64
	for _, q := range tokenize(query) {
		idf, ok := b.idf[q]
		if !ok {
			continue
		}

		freq := tf[q]
		num := freq * (b.k1 + 1)
		den := freq + b.k1*(1-b.b+b.b*(docLen/b.avgDocLen))

		score += idf * (num / den)
	}

	return score
}

// ScoreAndNormalize computes BM25 scores for all chunks and normalizes them
// to the range [0.0, 1.0] using min-max scaling across the current set.
// This improves score distribution compared to max-only normalization.
// It mutates the provided slice in place.
func (b *BM25) ScoreAndNormalize(query string, chunks []Chunk) {
	var max float64
	min := math.MaxFloat64

	for i := range chunks {
		s := b.Score(query, chunks[i])
		chunks[i].Score = s
		if s > max {
			max = s
		}
		if s < min {
			min = s
		}
	}

	if max == 0 {
		return
	}

	if max > min {
		for i := range chunks {
			chunks[i].Score = (chunks[i].Score - min) / (max - min)
		}
	}
}

// tokenize lowercases and splits on whitespace.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
			return r
		}
		return ' '
	}, s)
	return strings.Fields(s)
}
