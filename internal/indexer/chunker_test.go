package indexer

import "testing"

func Test_formatChunk(t *testing.T) {
	got := formatChunk("a/b/c.go", "func A() {}")
	want := "file: a/b/c.go\n\nfunc A() {}"
	if got != want {
		t.Fatalf("Expected %s, got %s", want, got)
	}

	got = formatChunk("a/b/c.go", "A does ...", "func A() {}")
	want = "file: a/b/c.go\n\nA does ...\nfunc A() {}"
	if got != want {
		t.Fatalf("Expected %s, got %s", want, got)
	}
}
