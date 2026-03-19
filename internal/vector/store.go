package vector

import (
	"container/heap"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"slices"

	_ "github.com/mattn/go-sqlite3"
)

const mmrOversampleFactor = 4

type Item struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
	Embedding []float64
}

type Result struct {
	Item
	Score float64
}

type Store struct {
	db          *sql.DB
	loaded      bool
	ProjectRoot string
	Items       []Item
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

// Search finds the top-k most similar chunks to the given query vector.
// The query vector is normalized internally and results are ranked using
// cosine similarity against the normalized embeddings stored in memory.
func (s *Store) Search(query []float64, k int) ([]Result, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}

	query = normalize(query)
	return s.topK(query, k), nil
}

// SearchMMR applies Max Marginal Relevance (MMR) to select k items from a candidate set
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
func (s *Store) SearchMMR(query []float64, k int, lambda float64) ([]Result, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}

	query = normalize(query)
	candidates := s.topK(query, k*mmrOversampleFactor)
	return mmr(query, candidates, k, lambda), nil
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

// ensureLoaded lazily loads the store from the DB only if Items is empty
// and the store has not already been loaded. This allows in-memory
// tests and temporary stores without hitting the DB.
func (s *Store) ensureLoaded() error {
	if !s.loaded && len(s.Items) == 0 {
		return s.Load()
	}
	return nil
}

func mmr(query []float64, results []Result, k int, lambda float64) []Result {
	selected := []Result{}
	candidates := make([]Item, len(results))
	for i, r := range results {
		candidates[i] = r.Item
	}

	for len(selected) < k && len(candidates) > 0 {
		var bestIdx int
		var bestScore = math.Inf(-1)

		for i, item := range candidates {
			simToQuery := cosine(query, item.Embedding)

			// Determine how similar this candidate to other picked items.
			var maxSimToSelected float64
			for _, sel := range selected {
				sim := cosine(item.Embedding, sel.Embedding)
				if sim > maxSimToSelected {
					maxSimToSelected = sim
				}
			}

			// Determine score based on relevace but penalise if it's too similar to what we already have.
			score := lambda*simToQuery - (1-lambda)*maxSimToSelected

			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		best := candidates[bestIdx]
		selected = append(selected, Result{
			Item:  best,
			Score: bestScore,
		})

		// remove selected item
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
