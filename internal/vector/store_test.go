package vector

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinResult(t *testing.T) {
	iA := Item{Content: "file: file_a.go (lines 1-1)\n\nfunc A()"}
	iB := Item{Content: "file: file_b.go (lines 1-1)\n\nfunc B()"}
	iC := Item{Content: "file: file_c.go (lines 1-1)\n\nfunc C()"}

	results := []Result{
		{Item: iA, Score: 0.879},
		{Item: iB, Score: 0.654},
		{Item: iC, Score: 0.342},
	}

	want := `
[1] (score: 0.879)
file: file_a.go (lines 1-1)

func A()

---

[2] (score: 0.654)
file: file_b.go (lines 1-1)

func B()

---

[3] (score: 0.342)
file: file_c.go (lines 1-1)

func C()

---
`

	assert.Equal(t, want, JoinResults(results...))
}

func TestNewStore(t *testing.T) {
	// Test initialisation with hash.
	pRoot := "/home/me/project/root"
	tmpDir := t.TempDir()
	store, err := NewStore(pRoot, tmpDir, "my_db", "hash")
	assert.NoError(t, err)
	defer store.Close()

	assert.NotNil(t, store)
	assert.NotNil(t, store.db)
	assert.Equal(t, store.DBPath, filepath.Join(tmpDir, "my_db.sqlite"))
	assert.Equal(t, store.ProjectRoot, pRoot)

	// Test initialisation with data.
	store.Add("file.go", "func A()", 1, 1, []float64{1, 0})
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
	store.Add("a.go", "func A()", 1, 1, []float64{1, 0})
	store.Add("b.go", "func B()", 1, 1, []float64{0, 1})
	store.Add("c.go", "func C()", 1, 1, []float64{0.8, 0.2})

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
				assert.Equal(t, tc.expectContent[i], results[i].Content)
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
		store.Add(
			fmt.Sprintf("file%d.go", i),
			"func test() {}",
			1, 1,
			randomVector(dim),
		)
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
