package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()

	// Create structure:
	//
	// /project
	//   main.go
	//   node_modules/
	//     lib.js
	//   src/
	//     app.go
	//   debug.log

	mustWriteFile := func(path string) {
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte("test"), 0644)
		require.NoError(t, err)
	}

	mustWriteFile(filepath.Join(dir, "main.go"))
	mustWriteFile(filepath.Join(dir, "node_modules/lib.js"))
	mustWriteFile(filepath.Join(dir, "src/app.go"))
	mustWriteFile(filepath.Join(dir, "debug.log"))
	mustWriteFile(filepath.Join(dir, "Dockerfile"))

	testCases := map[string]struct {
		ignore *Ignore
		ext    map[string]struct{}
		want   []string
	}{
		"skips ignored directories": {
			ignore: NewIgnoreFromPatterns("node_modules"),
			ext: map[string]struct{}{
				".go":  {},
				".js":  {},
				".log": {},
			},
			want: []string{
				"main.go",
				"app.go",
				"debug.log",
				"Dockerfile",
			},
		},
		"ignores file patterns": {
			ignore: NewIgnoreFromPatterns("node_modules", "*.log"),
			ext: map[string]struct{}{
				".go":  {},
				".js":  {},
				".log": {},
			},
			want: []string{
				"main.go",
				"app.go",
				"Dockerfile",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			files, err := Scan(dir, tc.ignore, tc.ext)
			require.NoError(t, err)

			// Normalize for comparison
			var names []string
			for _, f := range files {
				names = append(names, filepath.Base(f))
			}

			assert.ElementsMatch(t, tc.want, names)
		})
	}
}
