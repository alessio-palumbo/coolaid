package vector

import (
	"fmt"
	"math/rand/v2"
	"testing"
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
	if got := JoinResults(results...); got != want {
		t.Fatalf("expected %s got %s", got, want)
	}

}

func TestNewStore(t *testing.T) {
	// Test initialisation.
	store, tmpDir := newTestStore(t)
	if store == nil {
		t.Fatal("expected store to be initialized")
	}
	if store.db == nil {
		t.Fatal("expected database connection")
	}

	// Test initialisation with data.
	store.Add("file.go", "func A()", 1, 1, []float64{1, 0})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload store
	store2, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	store2.EnsureLoaded()
	if err != nil {
		t.Fatal(err)
	}
	if len(store2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store2.Items))
	}
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
			if len(results) != tc.expectN {
				t.Fatalf("expected %d result, got %d", tc.expectN, len(results))
			}
			for i := range results {
				if got, want := results[i].Content, tc.expectContent[i]; got != want {
					t.Fatalf("expected %s to rank %d, got %s", got, i, want)
				}
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
