package vector

import "testing"

func TestNewStore(t *testing.T) {
	// Test initialisation.
	store := newTestStore(t)
	if store == nil {
		t.Fatal("expected store to be initialized")
	}
	if store.db == nil {
		t.Fatal("expected database connection")
	}

	// Test initialisation with data.
	store.Add("file.go", "func A()", []float64{1, 0})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload store
	store2, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	if len(store2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store2.Items))
	}
}

func TestSearch(t *testing.T) {
	store := newTestStore(t)
	store.Add("a.go", "func A()", []float64{1, 0})
	store.Add("b.go", "func B()", []float64{0, 1})
	store.Add("c.go", "func C()", []float64{0.8, 0.2})

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
