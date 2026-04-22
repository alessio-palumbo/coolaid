package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbedPipeline(t *testing.T) {
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
		files int
	}{
		"single file":    {files: 1},
		"multiple files": {files: 3},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			llm := &fakeLLM{}
			store := &fakeStore{}

			var progressCalls int64
			onProgress := func(p Progress) {
				atomic.AddInt64(&progressCalls, 1)
			}

			p := NewEmbedPipeline(
				ctx,
				llm,
				store,
				slog.Default(),
				2,
				tc.files,
				onProgress,
			)

			files := make([]string, tc.files)
			root := t.TempDir()
			for i := range tc.files {
				file := filepath.Join(root, fmt.Sprintf("f%d.go", i))
				require.NoError(t, os.WriteFile(file, []byte(content), 0o644))
				files[i] = file
			}

			for _, f := range files {
				p.Submit(embedJob{
					file:    f,
					content: []byte(content),
				})
			}

			p.Wait()

			require.Greater(t, llm.calls, 0, "expected embeddings to be called")
			require.Greater(t, store.items, 0, "expected items stored")

			// critical invariant: one progress per file
			require.Equal(t, int64(tc.files), progressCalls, "progress should be emitted once per file")
		})
	}
}
