package indexer

import (
	"context"
	"coolaid/internal/store"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	content := `
	package main

	// This is a test file with enough content to exceed the minimum chunk size.
	// We repeat content to ensure chunking happens correctly and embeddings are triggered.

	func main() {
		println("hello world")
	}

	// Additional content to exceed fifty characters easily.
	// Lorem ipsum dolor sit amet, consectetur adipiscing elit.
	// Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
	`

	tests := map[string]struct {
		files             map[string]string
		extensions        map[string]struct{}
		expectedFiles     int
		expectSummary     bool
		expectEmbedCalled bool
	}{
		"indexes supported files only": {
			files: map[string]string{
				"main.go":    content,
				"readme.md":  content,
				"ignore.txt": "nope",
			},
			extensions: map[string]struct{}{
				".go": {},
				".md": {},
			},
			expectedFiles:     2,
			expectSummary:     true,
			expectEmbedCalled: true,
		},
		"no matching extensions triggers early exit": {
			files: map[string]string{
				"file.txt": content,
			},
			extensions: map[string]struct{}{
				".go": {},
			},
			expectedFiles:     0,
			expectSummary:     false,
			expectEmbedCalled: false,
		},
		"empty project": {
			files: map[string]string{},
			extensions: map[string]struct{}{
				".go": {},
			},
			expectedFiles:     0,
			expectSummary:     false,
			expectEmbedCalled: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			// create files
			for path, content := range tc.files {
				full := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			llm := &fakeLLM{}
			store := &fakeStore{}
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			opts := IndexOptions{
				ProjectRoot: root,
				Extensions:  tc.extensions,
				MaxWorkers:  1, // deterministic
			}
			err := Build(context.Background(), llm, store, logger, opts, nil)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			if tc.expectedFiles == 0 {
				require.Equal(t, 0, llm.calls, "expected no embedding calls when no files are indexed")
				require.Equal(t, 0, store.items, "expected no items stored when no files are indexed")
				require.Empty(t, store.summary, "expected no summary when no files are indexed")
			} else {
				require.Greater(t, llm.calls, 0, "expected at least one embedding call")
				require.Greater(t, store.items, 0, "expected items to be stored")
				require.NotEmpty(t, store.summary, "expected summary to be generated")
			}
		})
	}

}

type fakeLLM struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeLLM) Embed(ctx context.Context, text string) ([]float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return []float64{1.0, 2.0, 3.0}, nil
}

type fakeStore struct {
	mu      sync.Mutex
	items   int
	summary string
}

func (f *fakeStore) AddItem(i store.Item) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items++
}

func (f *fakeStore) AddSummary(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summary = s
}
