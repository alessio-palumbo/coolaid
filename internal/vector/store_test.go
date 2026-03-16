package vector

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

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

	if len(store2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store2.Items))
	}
}

func TestSearch(t *testing.T) {
	store := &Store{}
	store.Add("a.go", "func A()", 1, 1, []float64{1, 0})
	store.Add("b.go", "func B()", 1, 1, []float64{0, 1})
	store.Add("c.go", "func C()", 1, 1, []float64{0.8, 0.2})

	query := []float64{1, 0}
	results := store.Search(query, 1)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Content != "func A()" {
		t.Fatalf("unexpected result: %s", results[0].Content)
	}

	results = store.Search(query, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expect := []string{"func A()", "func C()", "func B()"}
	for i := range results {
		if got, want := results[i].Content, expect[i]; got != want {
			t.Fatalf("expected %s to rank %d, got %s", got, i, want)
		}
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
		store.Search(query, 5)
	}
}

func randomVector(dim int) []float64 {
	v := make([]float64, dim)
	for i := range v {
		v[i] = rand.Float64()
	}
	return normalize(v)
}
