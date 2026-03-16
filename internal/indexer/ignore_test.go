package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatch(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()

	// create project .gitignore
	err := os.WriteFile(
		filepath.Join(project, ".gitignore"),
		[]byte("vendor/\n*.log\n"),
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

	// create global ignore
	err = os.WriteFile(
		filepath.Join(config, "ignore"),
		[]byte("*.tmp\n"),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	ignore, err := LoadIgnore(project, config)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		ignored bool
	}{
		{"vendor/foo.go", true},
		{"pkg/main.go", false},
		{"app.log", true},
		{"docs/readme.md", true},
		{"build.tmp", true},
		{"cmd/server.go", false},
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
