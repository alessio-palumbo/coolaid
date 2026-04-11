package vector

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStore(t *testing.T) {
	// Test initialisation with hash.
	pRoot := "/home/me/project/root"
	tmpDir := t.TempDir()
	store, err := NewStore(pRoot, tmpDir, "my_db", "hash")
	assert.NoError(t, err)
	defer store.Close()
	assert.NoError(t, store.ResetIndex())

	assert.NotNil(t, store)
	assert.NotNil(t, store.db)
	assert.Equal(t, store.DBPath, filepath.Join(tmpDir, "my_db.sqlite"))
	assert.Equal(t, store.ProjectRoot, pRoot)

	// Test initialisation with data.
	store.AddItem(Item{FilePath: "file.go", Content: "func A()", StartLine: 1, EndLine: 1, Embedding: []float64{1, 0}})
	assert.NoError(t, store.Save())

	// Reload store, same hash.
	store2, err := NewStore(pRoot, tmpDir, "my_db", "hash")
	assert.NoError(t, err)
	defer store2.Close()

	assert.NoError(t, store2.EnsureLoaded())
	assert.Equal(t, len(store2.Items), 1)

	// Reload store, hash changed.
	store2, err = NewStore(pRoot, tmpDir, "my_db", "new_hash")
	assert.NoError(t, err)
	defer store2.Close()

	assert.Error(t, ErrReindexRequired, err)
	assert.Equal(t, len(store2.Items), 0)
}

func TestSearch(t *testing.T) {
	store := &Store{}
	store.AddItem(Item{FilePath: "a.go", Content: "func A()", StartLine: 1, EndLine: 1, Embedding: []float64{1, 0}})
	store.AddItem(Item{FilePath: "b.go", Content: "func B()", StartLine: 1, EndLine: 1, Embedding: []float64{0, 1}})
	store.AddItem(Item{FilePath: "c.go", Content: "func C()", StartLine: 1, EndLine: 1, Embedding: []float64{0.8, 0.2}})

	testCases := map[string]struct {
		k             int
		useMMR        bool
		expectN       int
		expectContent []string
	}{
		"single": {
			k:             1,
			expectN:       1,
			expectContent: []string{"func A()"},
		},
		"multiple": {
			k:             3,
			expectN:       3,
			expectContent: []string{"func A()", "func C()", "func B()"},
		},
		"single: mmr": {
			k:             1,
			useMMR:        true,
			expectN:       1,
			expectContent: []string{"func A()"},
		},
		"multiple: mmr": {
			k:             3,
			useMMR:        true,
			expectN:       3,
			expectContent: []string{"func A()", "func C()", "func B()"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			query := []float64{1, 0}
			results, _ := store.Search(query, tc.k, false)
			assert.Equal(t, tc.expectN, len(results))
			for i := range results {
				assert.Equal(t, tc.expectContent[i], results[i].Text)
			}
		})
	}
}

func BenchmarkSearch(b *testing.B) {
	var (
		dim    = 768
		chunks = 5000
	)

	store := &Store{Items: make([]Item, 0, chunks)}

	// populate store
	for i := range chunks {
		store.AddItem(Item{
			FilePath:  fmt.Sprintf("file%d.go", i),
			Content:   "func test() {}",
			StartLine: 1, EndLine: 1,
			Embedding: randomVector(dim),
		})
	}

	query := randomVector(dim)
	b.ResetTimer()
	for b.Loop() {
		store.Search(query, 5, false)
	}
}

func randomVector(dim int) []float64 {
	v := make([]float64, dim)
	for i := range v {
		v[i] = rand.Float64()
	}
	return normalize(v)
}
