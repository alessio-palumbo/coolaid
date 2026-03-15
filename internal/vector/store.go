package vector

import (
	"cmp"
	"database/sql"
	"os"
	"slices"

	_ "github.com/mattn/go-sqlite3"
)

var defaultDBPath = ".ai/index.db"

type Item struct {
	FilePath  string
	Content   string
	Embedding []float64
}

type Result struct {
	Item
	Score float64
}

type Store struct {
	db    *sql.DB
	Items []Item
}

// NewStore creates and initializes a vector Store backed by SQLite.
// It opens the database, ensures the required tables exist, and loads
// the stored embeddings into memory so they can be searched efficiently.
func NewStore() (*Store, error) {
	db, err := openDefaultDB()
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.Load(); err != nil {
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
func (s *Store) Add(path, chunk string, emb []float64) {
	s.Items = append(s.Items, Item{
		FilePath:  path,
		Content:   chunk,
		Embedding: normalize(emb),
	})
}

// Search finds the top-k most similar chunks to the given query vector.
// The query vector is normalized internally and results are ranked using
// cosine similarity against the normalized embeddings stored in memory.
func (s *Store) Search(query []float64, k int) []Result {
	query = normalize(query)
	results := make([]Result, 0, len(s.Items))
	for _, item := range s.Items {
		results = append(results, Result{
			Item:  item,
			Score: cosine(query, item.Embedding),
		})
	}

	slices.SortFunc(results, func(i, j Result) int {
		return cmp.Compare(j.Score, i.Score)
	})

	if len(results) > k {
		results = results[:k]
	}
	return results
}

func openDefaultDB() (*sql.DB, error) {
	err := os.MkdirAll(".ai", 0755)
	if err != nil {
		return nil, err
	}

	return sql.Open("sqlite3", defaultDBPath)
}
