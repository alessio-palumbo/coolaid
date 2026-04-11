package retrieval

import (
	"container/heap"
	"testing"
)

func TestChunkHeap_DrainDesc(t *testing.T) {
	tests := []struct {
		name   string
		input  []Chunk
		expect []float64
	}{
		{
			name: "basic ordering",
			input: []Chunk{
				{Score: 0.2},
				{Score: 0.9},
				{Score: 0.5},
				{Score: 0.7},
			},
			expect: []float64{0.9, 0.7, 0.5, 0.2},
		},
		{
			name: "single element",
			input: []Chunk{
				{Score: 0.42},
			},
			expect: []float64{0.42},
		},
		{
			name:   "empty",
			input:  nil,
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &ChunkHeap{}
			heap.Init(h)

			for _, c := range tt.input {
				heap.Push(h, c)
			}

			out := h.DrainDesc()

			if len(out) != len(tt.expect) {
				t.Fatalf("expected len %d, got %d", len(tt.expect), len(out))
			}

			for i, c := range out {
				if c.Score != tt.expect[i] {
					t.Fatalf("at %d: expected %.2f, got %.2f", i, tt.expect[i], c.Score)
				}
			}
		})
	}
}
