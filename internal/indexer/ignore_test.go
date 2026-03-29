package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatch(t *testing.T) {
	project := t.TempDir()

	// create project .gitignore
	err := os.WriteFile(
		filepath.Join(project, ".gitignore"),
		[]byte("vendor/\n*.log\nvenv\napp\n!app/"),
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
		isDir   bool
		ignored bool
	}{
		// defaults (.git/, node_modules/, vendor/, dist/, build/)
		{".git/hooks", true, true},
		{"node_modules", true, true},
		{"src/node_modules/foo.go", false, true},
		{"src/node_modules", true, true},

		// vendor
		{"vendor/foo.go", false, true},

		// build/dist defaults
		{"build/app.bin", false, true},
		{"dist/tmp.txt", false, true},

		// lock + tmp
		{"package.lock", false, true},
		{"build.tmp", false, true},

		// .gitignore patterns
		{"app", false, true},            // matches "app"
		{"app", true, false},            // negated by "!app/"
		{"app/readme.md", false, false}, // unignored
		{"app.log", false, true},        // *.log

		// .aiignore
		{"docs/readme.md", false, true},

		// normal files
		{"pkg/main.go", false, false},
		{"cmd/server.go", false, false},

		// tmp override
		{"something.tmp", false, true},

		// venv
		{"venv/lib/python3.12/site-packages/PIL/AvifImagePlugin.py", false, true},

		// compound path tests
		{"src/app/main.go", false, false},
		{"src/app/main.go", true, false},
		{"src/app/", true, false},
		{"src/app", false, true}, // important: file should be ignored due to "app"

		// glob tests
		{"*.log", false, true},
		{"debug.log", false, true},
		{"logs/debug.log", false, true},
		{"logs/debug.txt", false, false},
	}

	for _, tt := range tests {
		got := ignore.Match(tt.path, tt.isDir)
		if got != tt.ignored {
			t.Errorf(
				"path %s (isDir=%v) expected ignored=%v got=%v",
				tt.path,
				tt.isDir,
				tt.ignored,
				got,
			)
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	ig, _ := LoadIgnore(".", nil)

	paths := make([]string, 0, 100)
	for i := range 50 {
		paths = append(paths,
			fmt.Sprintf("src/file%d.go", i),
			fmt.Sprintf("node_modules/pkg%d/index.js", i),
			fmt.Sprintf("deep/path/%d/%d/%d/file.txt", i, i, i),
		)
	}

	b.ResetTimer()
	for b.Loop() {
		for _, p := range paths {
			ig.Match(p, false)
		}
	}
}
