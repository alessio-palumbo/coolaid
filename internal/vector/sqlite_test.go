package vector

import (
	"testing"
)

func TestStoreSaveLoad(t *testing.T) {
	store, tmpDir := newTestStore(t)
	store.Add("file.go", "func test()", 1, 1, []float64{1, 2, 3})
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	store2, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	if len(store2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store2.Items))
	}
}

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store, tmpDir
}
