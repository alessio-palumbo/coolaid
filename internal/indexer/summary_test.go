package indexer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummaryBuilder(t *testing.T) {
	tests := map[string]struct {
		files          map[string]string
		expectReadme   bool
		expectSymbols  bool
		expectFileTrim bool
	}{
		"basic summary": {
			files: map[string]string{
				"main.go": "package main\nfunc main() {}",
			},
			expectReadme:   false,
			expectSymbols:  true,
			expectFileTrim: false,
		},
		"includes readme": {
			files: map[string]string{
				"README.md": "This is a project\n" + strings.Repeat("a", 2000),
			},
			expectReadme:   true,
			expectSymbols:  false,
			expectFileTrim: false,
		},
		"multiple files capped": {
			files: func() map[string]string {
				m := make(map[string]string)
				for i := range 30 {
					m[fmt.Sprintf("f%d.go", i)] = "package main"
				}
				return m
			}(),
			expectReadme:   false,
			expectSymbols:  false,
			expectFileTrim: true,
		},
		"readme only taken once": {
			files: map[string]string{
				"README.md":      "first",
				"docs/readme.md": "second",
			},
			expectReadme:   true,
			expectSymbols:  false,
			expectFileTrim: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := NewSummaryBuilder()

			for path, content := range tc.files {
				b.AddFile(path, []byte(content))
			}

			out := b.Build()

			require.Contains(t, out, "Files:\n")
			require.Contains(t, out, "Symbols:\n")

			if tc.expectReadme {
				require.Contains(t, out, "README:\n")
			} else {
				require.NotContains(t, out, "README:\n")
			}

			if tc.expectFileTrim {
				lines := strings.Count(out, "- ")
				require.LessOrEqual(t, lines, tokenTypeMaxItems*2) // files + symbols
			}
		})
	}

	t.Run("readme truncated", func(t *testing.T) {
		b := NewSummaryBuilder()

		content := strings.Repeat("a", 2000)
		b.AddFile("README.md", []byte(content))

		out := b.Build()

		idx := strings.Index(out, "README:\n")
		require.NotEqual(t, -1, idx, "README section not found")

		readme := out[idx+len("README:\n"):]

		require.LessOrEqual(t, len(readme), readmeFragmentMaxSize)
		require.True(t, strings.HasPrefix(readme, content[:readmeFragmentMaxSize]))
		require.Equal(t, content[:readmeFragmentMaxSize], readme)
	})
}
