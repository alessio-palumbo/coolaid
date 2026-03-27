package indexer

import (
	"strings"
	"testing"
)

func Test_formatChunk(t *testing.T) {
	got := formatChunk("a/b/c.go", 1, 1, []byte("func A() {}"))
	want := "file: a/b/c.go (lines 1-1)\n\nfunc A() {}"
	if got != want {
		t.Fatalf("Expected %s, got %s", want, got)
	}

	got = formatChunk("a/b/c.go", 1, 1, []byte("// A does ...\nfunc A() {}"))
	want = "file: a/b/c.go (lines 1-1)\n\n// A does ...\nfunc A() {}"
	if got != want {
		t.Fatalf("Expected %s, got %s", want, got)
	}
}

func BenchmarkChunkGo(b *testing.B) {
	// simulate a file with 10k lines, ~80 chars per line
	var sb strings.Builder
	for range 100000 {
		sb.WriteString("This is a line in a large file to test chunking performance.\n")
	}
	content := []byte(sb.String())

	b.ResetTimer()
	for b.Loop() {
		ChunkText("dummy.txt", content)
	}
}

func BenchmarkChunkText(b *testing.B) {
	// simulate a file with 10k lines, ~80 chars per line
	var sb strings.Builder
	for range 100000 {
		sb.WriteString("This is a line in a large file to test chunking performance.\n")
	}
	content := []byte(sb.String())

	b.ResetTimer()
	for b.Loop() {
		ChunkText("dummy.txt", content)
	}
}
