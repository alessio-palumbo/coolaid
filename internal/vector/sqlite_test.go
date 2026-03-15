package vector

import (
	"path/filepath"
	"testing"
)

func TestStoreSaveLoad(t *testing.T) {
	store := newTestStore(t)
	store.Add("file.go", "func test()", []float64{1, 2, 3})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	store2, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}

	if len(store2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store2.Items))
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	db := filepath.Join(t.TempDir(), "test.db")
	old := defaultDBPath
	defaultDBPath = db
	t.Cleanup(func() { defaultDBPath = old })

	store, err := NewStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
