package indexer

import (
	"testing"
)

func TestChunkGo(t *testing.T) {
	src := `
	    package main

	    func A() {}

	    func B() {}
	`

	chunks := ChunkGo(src)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(chunks))
	}
}
