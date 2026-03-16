package indexer

import (
	"testing"
)

func TestChunkGo(t *testing.T) {
	path := "a/b/c.go"
	src := `
	    package main

	    func A() {}

	    func B() {}
	`

	chunks := ChunkGo(path, src)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(chunks))
	}
}
