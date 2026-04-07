package vector

import (
	"cmp"
	"container/heap"
	"coolaid/internal/retrieval"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// defaultMMRLambda controls the tradeoff between relevance (1) and diversity (0).
const defaultMMRLambda = 0.85

// mmrOversampleFactor is the k-multiplier used to search for candidates.
const mmrOversampleFactor = 4

// Item represents a Chunk and it's metadata.
type Item struct {
	FilePath  string
	Symbol    string
	Kind      string
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
	db         *sql.DB
	now        func() time.Time
	loaded     bool
	configHash string

	ProjectRoot string
	DBPath      string
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

// ToContextChunks converts vector search results into retrieval.Chunk values
// suitable for LLM prompting. It maps content, source, and score into a
// unified format consumed by downstream components.
func ToContextChunks(results ...Result) []retrieval.Chunk {
	var out []retrieval.Chunk
	for _, r := range results {
		out = append(out, retrieval.Chunk{
			Text:   r.Content,
			Source: r.FilePath,
			Score:  r.Score,
		})
	}
	return out
}

// NewStore creates and initializes a vector Store backed by SQLite.
// It opens the database, ensures the required tables exist, and loads
// the stored embeddings into memory so they can be searched efficiently.
func NewStore(projectRoot, storeDir, dbName, configHash string) (*Store, error) {
	dbPath := filepath.Join(storeDir, dbName+".sqlite")
	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	s := &Store{
		db:          db,
		now:         func() time.Time { return time.Now().UTC() },
		configHash:  configHash,
		ProjectRoot: projectRoot,
		DBPath:      dbPath,
	}
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

// AddItem adds a chunk to the in-memory index.
// The embedding is normalized so that cosine
// similarity can be computed efficiently during search.
func (s *Store) AddItem(i Item) {
	i.Embedding = normalize(i.Embedding)
	s.Items = append(s.Items, i)
}

// AddSummary adds a summary to the in-memory Store.
func (s *Store) AddSummary(summary string) {
	s.Summary = summary
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
	if k <= 0 {
		return nil, fmt.Errorf("invalid value for k: [%d]", k)
	}
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

// FindBySymbol searches the in-memory index for chunks matching a given symbol.
//
// This provides a fast-path for identifier-based queries (e.g. "TestCommand"),
// avoiding database access and returning precise, deterministic matches.
//
// Matching is exact and case-sensitive. Results are returned in insertion order
// and limited by the provided 'limit'.
func (s *Store) FindBySymbol(query, sym string, limit int) ([]Result, error) {
	if err := s.EnsureLoaded(); err != nil {
		return nil, err
	}

	nSym := normalizeSymbol(sym)
	var results []Result
	for _, it := range s.Items {
		if it.Symbol == nSym {
			results = append(results, Result{
				Item:  it,
				Score: scoreSymbol(it, query),
			})
			if len(results) >= limit {
				break
			}
		}
	}

	// Sort in descending order.
	slices.SortFunc(results, func(a, b Result) int {
		return cmp.Compare(b.Score, a.Score)
	})
	return results, nil
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

func openDB(path string) (*sql.DB, error) {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite3", path)
}

// scoreSymbol assigns a relevance score to a symbol-matched item.
//
// Symbol matches are treated as high-confidence signals and start with a high base score.
// An additional boost is applied for exact matches (defensive, in case of future fuzzy matching).
//
// A small penalty is applied based on chunk length to prefer more focused (shorter) code blocks,
// which tend to produce more precise LLM outputs.
//
// Note: This scoring is intentionally simple and only used to rank multiple symbol matches.
// It is not comparable to cosine similarity scores from vector search.
func scoreSymbol(item Item, query string) float64 {
	score := 1.0

	if item.Symbol == query {
		score += 1.0 // exact match boost
	}

	// optional: prefer shorter chunks (more focused)
	length := item.EndLine - item.StartLine
	score -= float64(length) * 0.001

	return score
}

// normalizeSymbol extracts the base identifier from a possibly qualified symbol.
//
// It strips any prefix before the last '.', allowing inputs like "pkg.Func"
// to match stored symbols like "Func". If no qualifier is present, the input
// is returned unchanged.
//
// This provides a lightweight, language-agnostic way to improve symbol matching
// without requiring explicit module/package support.
func normalizeSymbol(s string) string {
	if i := strings.LastIndex(s, "."); i != -1 {
		return s[i+1:]
	}
	return s
}
