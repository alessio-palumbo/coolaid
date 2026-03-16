package vector

import (
	"container/heap"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

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
func (s *Store) Search(query []float64, k int) []Result {
	query = normalize(query)
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
