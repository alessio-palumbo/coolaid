package vector

import (
	"testing"
)

func TestStoreSaveLoad(t *testing.T) {
	store, tmpDir := newTestStore(t)
	store.Add("file.go", "func test()", 1, 1, []float64{1, 2, 3})
	store.AddSummary("summary")
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	store2, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	store2.EnsureLoaded()
	if len(store2.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(store2.Items))
	}
	if store2.Summary != "summary" {
		t.Fatalf("Expected summary to be loaded")
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
