package store

import (
	"cmp"
	"container/heap"
	"coolaid/internal/retrieval"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// mmrLambda controls the tradeoff between relevance (1) and diversity (0).
const mmrLambda = 0.85

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

	memory Memory
}

// Memory is the persisted, compact representation of project-level context.
//
// It evolves over time based on extracted signals from user interactions
// and is used to improve future LLM responses with stable intent, topics,
// and preferences.
type Memory struct {
	ProjectSummary string
	Topics         []string
	Preferences    []string
	UpdatedAt      time.Time
}

// NewStore creates and initializes a Store backed by SQLite.
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
func (s *Store) Search(query []float64, k int, useMMR bool) ([]retrieval.Chunk, error) {
	if k <= 0 {
		return nil, fmt.Errorf("invalid value for k: [%d]", k)
	}
	if err := s.EnsureLoaded(); err != nil {
		return nil, err
	}

	if useMMR {
		candidates := s.topKFromItems(query, k*mmrOversampleFactor)
		return retrieval.MMR(candidates, k, mmrLambda, embeddingSim), nil
	}
	return s.topKFromItems(query, k), nil
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
func (s *Store) FindBySymbol(query, sym string, limit int) ([]retrieval.Chunk, error) {
	if err := s.EnsureLoaded(); err != nil {
		return nil, err
	}

	nSym := normalizeSymbol(sym)
	var chunks []retrieval.Chunk
	for _, it := range s.Items {
		if it.Symbol == nSym {
			chunks = append(chunks, retrieval.Chunk{
				Text:      it.Content,
				Source:    it.FilePath,
				Embedding: it.Embedding,
				Score:     scoreSymbol(it, query),
			})
			if len(chunks) >= limit {
				break
			}
		}
	}

	// Sort in descending order.
	slices.SortFunc(chunks, func(a, b retrieval.Chunk) int {
		return cmp.Compare(b.Score, a.Score)
	})
	return chunks, nil
}

func (s *Store) topKFromItems(query []float64, k int) []retrieval.Chunk {
	if len(s.Items) == 0 || k <= 0 {
		return nil
	}

	h := &retrieval.ChunkHeap{}
	heap.Init(h)

	query = normalize(query)

	for _, item := range s.Items {
		score := cosine(query, item.Embedding)
		c := retrieval.Chunk{
			Text:      item.Content,
			Source:    item.FilePath,
			Score:     score,
			Embedding: item.Embedding,
		}

		if h.Len() < k {
			heap.Push(h, c)
			continue
		}
		if score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, c)
		}
	}

	return h.DrainDesc()
}

func embeddingSim(a, b retrieval.Chunk) float64 {
	return cosine(a.Embedding, b.Embedding)
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
