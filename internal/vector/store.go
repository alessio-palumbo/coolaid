package vector

import (
	"container/heap"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"

	_ "github.com/mattn/go-sqlite3"
)

const (
	SearchModeFast     string = "fast"
	SearchModeBalanced string = "balanced"
	SearchModeDeep     string = "deep"
)

// defaultMMRLambda controls the tradeoff between relevance (1) and diversity (0).
const defaultMMRLambda = 0.85

// mmrOversampleFactor is the k-multiplier used to search for candidates.
const mmrOversampleFactor = 4

// Item represents a Chunk and it's metadata.
type Item struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
	Embedding []float64
}

// Result combines an Item with its search Score.
type Result struct {
	Item
	Score float64
}

// Store allows storage and retrieval of Items and Summary
// for an indexed repository or folder.
type Store struct {
	db          *sql.DB
	loaded      bool
	ProjectRoot string
	Items       []Item
	Summary     string
}

// JoinResults formats returns a formatted string of results printing or for LLM prompting.
// It prepends ranking and score as metadata and assumes file and lines are
// already present at the top of each chunk (Item.Content).
func JoinResults(results ...Result) string {
	var out string
	for i, r := range results {
		out += fmt.Sprintf(
			"\n[%d] (score: %.3f)\n%s\n\n---\n",
			i+1,
			r.Score,
			r.Content,
		)
	}
	return out
}

// NewStore creates and initializes a vector Store backed by SQLite.
// It opens the database, ensures the required tables exist, and loads
// the stored embeddings into memory so they can be searched efficiently.
func NewStore(indexesDir string) (*Store, error) {
	projectRoot, err := projectRoot()
	if err != nil {
		return nil, err
	}

	db, err := openDB(indexesDir, projectRoot)
	if err != nil {
		return nil, err
	}

	s := &Store{db: db, ProjectRoot: projectRoot}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying DB.
func (s *Store) Close() error {
	return s.db.Close()
}

// Add adds a chunk to the in-memory index.
// The embedding is normalized so that cosine
// similarity can be computed efficiently during search.
func (s *Store) Add(path, text string, startLine, endLine int, emb []float64) {
	s.Items = append(s.Items, Item{
		FilePath:  path,
		StartLine: startLine,
		EndLine:   endLine,
		Content:   text,
		Embedding: normalize(emb),
	})
}

// AddSummary adds a summary to the in-memory Store.
func (s *Store) AddSummary(summary string) {
	s.Summary = summary
}

// SearchForMode performs a Search according to named configurations.
func (s *Store) SearchForMode(mode string, queryVec []float64) ([]Result, error) {
	switch mode {
	case SearchModeDeep:
		return s.Search(queryVec, 12, true)
	case SearchModeBalanced:
		return s.Search(queryVec, 8, false)
	default:
		return s.Search(queryVec, 5, false)
	}
}

// Search finds the top-k most similar chunks to the given query vector.
// The query vector is normalized internally and results are ranked using
// cosine similarity against the normalized embeddings stored in memory.

// If useMMR is true it then applies Max Marginal Relevance (MMR) to the candidate set
// of sizez k*mmrOversampleFactor.
//
// MMR balances two objectives:
//  1. Relevance to the query (via cosine similarity)
//  2. Diversity from already selected results
//
// The scoring function is:
//
//	score = λ * sim(query, candidate) - (1 - λ) * max_sim(candidate, selected)
//
// where:
//   - λ (lambda) controls the tradeoff between relevance and diversity
//   - sim() is cosine similarity
//
// Higher λ → more relevance (less diversity)
// Lower λ → more diversity (less relevance)
//
// This helps avoid returning many similar chunks (e.g. from the same file).
func (s *Store) Search(query []float64, k int, useMMR bool) ([]Result, error) {
	if err := s.EnsureLoaded(); err != nil {
		return nil, err
	}

	query = normalize(query)
	if useMMR {
		candidates := s.topK(query, k*mmrOversampleFactor)
		return mmr(candidates, k, defaultMMRLambda), nil
	}
	return s.topK(query, k), nil
}

// EnsureLoaded lazily loads the store from the DB only if Items is empty
// and the store has not already been loaded. This allows in-memory
// tests and temporary stores without hitting the DB.
func (s *Store) EnsureLoaded() error {
	if !s.loaded && len(s.Items) == 0 {
		return s.Load()
	}
	return nil
}

func (s *Store) topK(query []float64, k int) []Result {
	h := &resultHeap{}
	heap.Init(h)

	for _, item := range s.Items {
		score := cosine(query, item.Embedding)
		r := Result{
			Item:  item,
			Score: score,
		}

		if h.Len() < k {
			heap.Push(h, r)
			continue
		}
		if score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, r)
		}
	}

	results := make([]Result, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(Result)
	}
	return results
}

func mmr(results []Result, k int, lambda float64) []Result {
	selected := make([]Result, 0, k)
	candidates := make([]Result, len(results))
	copy(candidates, results)

	for len(selected) < k && len(candidates) > 0 {
		var bestIdx int
		bestScore := math.Inf(-1)

		for i, cand := range candidates {
			// Determine how similar this candidate to other picked items.
			var maxSimToSelected float64
			for _, sel := range selected {
				sim := cosine(cand.Embedding, sel.Embedding)
				if sim > maxSimToSelected {
					maxSimToSelected = sim
				}
			}

			// Determine score based on relevace but penalise if it's too similar to what we already have.
			score := lambda*cand.Score - (1-lambda)*maxSimToSelected

			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		selected = append(selected, candidates[bestIdx])
		// remove selected candidate
		candidates = slices.Delete(candidates, bestIdx, bestIdx+1)
	}

	return selected
}

func openDB(indexesDir, projectRoot string) (*sql.DB, error) {
	hash := sha1.Sum([]byte(projectRoot))
	name := hex.EncodeToString(hash[:8])
	path := filepath.Join(indexesDir, name+".sqlite")

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite3", path)
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		git := filepath.Join(dir, ".git")
		if _, err := os.Stat(git); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return dir, nil // reached filesystem root
		}

		dir = parent
	}
}
