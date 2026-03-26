package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatch(t *testing.T) {
	project := t.TempDir()

	// create project .gitignore
	err := os.WriteFile(
		filepath.Join(project, ".gitignore"),
		[]byte("vendor/\n*.log\nvenv\n"),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	// create project .aiignore
	err = os.WriteFile(
		filepath.Join(project, ".aiignore"),
		[]byte("docs/\n"),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	ignore, err := LoadIgnore(project, []string{"*.tmp"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		ignored bool
	}{
		{".git/hooks", true},
		{"node_modules", true},
		{"dist/tmp.txt", true},
		{"package.lock", true},
		{"build/app.bin", true},
		{"vendor/foo.go", true},
		{"pkg/main.go", false},
		{"app.log", true},
		{"docs/readme.md", true},
		{"build.tmp", true},
		{"cmd/server.go", false},
		{"venv/lib/python3.12/site-packages/PIL/AvifImagePlugin.py", true},
	}

	for _, tt := range tests {
		got := ignore.Match(tt.path)
		if got != tt.ignored {
			t.Errorf(
				"path %s expected ignored=%v got=%v",
				tt.path,
				tt.ignored,
				got,
			)
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	ig, _ := LoadIgnore(".", nil)

	// Test cases: hits, misses, and deep paths
	paths := []string{
		"src/main.go",                  // Miss
		"node_modules/lodash/index.js", // Hit (Dir pattern)
		"dist/bin/app",                 // Hit (Full path)
		"build.log",                    // Hit (Glob)
		"very/deeply/nested/path/to/some/file/no/match.txt", // Miss (Performance killer)
	}

	b.ResetTimer()
	for b.Loop() {
		for _, p := range paths {
			ig.Match(p)
		}
	}
}
