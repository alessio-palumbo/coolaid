package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkGo(t *testing.T) {
	path := "a/b/c.go"
	src := []byte(`
	    package main

	    func A() {}

	    func B() {}
	`)

	chunks := ChunkGo(path, src)
	assert.Equal(t, 2, len(chunks))
	c0 := Chunk{StartLine: 4, EndLine: 4, Symbol: "A", Kind: "function", Text: "File: a/b/c.go\nSymbol: A\nKind: function\nLines: 4-4\n\nfunc A() {}"}
	assert.Equal(t, c0, chunks[0])
	c1 := Chunk{StartLine: 6, EndLine: 6, Symbol: "B", Kind: "function", Text: "File: a/b/c.go\nSymbol: B\nKind: function\nLines: 6-6\n\nfunc B() {}"}
	assert.Equal(t, c1, chunks[1])
}
