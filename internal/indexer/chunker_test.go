package indexer

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkText_BasicSplitting(t *testing.T) {
	path := "test.txt"
	// Create content larger than defaultChunkCharacters
	line := "abcdefghijklmnopqrstuvwxyz\n" // 27 chars
	content := strings.Repeat(line, 30)    // ~810 chars

	chunks := ChunkText(path, []byte(content))
	// Should produce multiple chunks
	assert.Greater(t, len(chunks), 1)

	for _, c := range chunks {
		// Basic sanity checks
		assert.NotEmpty(t, c.Text)
		assert.GreaterOrEqual(t, c.StartLine, 1)
		assert.GreaterOrEqual(t, c.EndLine, c.StartLine)

		// Ensure formatting includes file info
		assert.Contains(t, c.Text, path)
	}
}

func TestChunkText_EmptyInput(t *testing.T) {
	chunks := ChunkText("test.txt", []byte{})
	assert.Nil(t, chunks)
}

func TestChunkText_MinChunkSize(t *testing.T) {
	path := "test.txt"
	// Smaller than minChunkSize
	content := strings.Repeat("a", minChunkSize-1)
	chunks := ChunkText(path, []byte(content))

	// Should not create a chunk
	assert.Len(t, chunks, 0)
}

func Test_formatChunk(t *testing.T) {
	got := formatChunk("a/b/c.go", 1, 1, nil, []byte("func A() {}"))
	want := "File: a/b/c.go\nLines: 1-1\n\nfunc A() {}"
	assert.Equal(t, want, got)

	fn := &ast.FuncDecl{Name: &ast.Ident{Name: "Chunker"}}
	got = formatChunk("a/b/c.go", 1, 1, fn, []byte("// A does ...\nfunc A() {}"))
	want = "File: a/b/c.go\nSymbol: Chunker\nKind: function\nLines: 1-1\n\n// A does ...\nfunc A() {}"
	assert.Equal(t, want, got)
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

func Benchmark_formatChunk(b *testing.B) {
	fn := &ast.FuncDecl{Name: &ast.Ident{Name: "Chunker"}}

	b.ResetTimer()
	for b.Loop() {
		formatChunk("a/b/c.go", 1, 1, fn, []byte("// A does ...\nfunc A() {}"))
	}
}
