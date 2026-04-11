package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinChunks(t *testing.T) {
	results := []Chunk{
		{Text: "file: file_a.go (lines 1-1)\n\nfunc A()", Score: 0.879},
		{Text: "file: file_b.go (lines 1-1)\n\nfunc B()", Score: 0.654},
		{Text: "file: file_c.go (lines 1-1)\n\nfunc C()", Score: 0.342},
	}

	want := `
[1] (score: 0.879)
file: file_a.go (lines 1-1)

func A()

---

[2] (score: 0.654)
file: file_b.go (lines 1-1)

func B()

---

[3] (score: 0.342)
file: file_c.go (lines 1-1)

func C()

---
`

	assert.Equal(t, want, JoinChunks(results...))
}

func TestTopK(t *testing.T) {
	chunks := []Chunk{
		{Score: 0.1},
		{Score: 0.9},
		{Score: 0.3},
		{Score: 0.8},
		{Score: 0.2},
	}

	out := TopK(chunks, 3)

	expected := []float64{0.9, 0.8, 0.3}

	for i := range expected {
		if out[i].Score != expected[i] {
			t.Fatalf("expected %.2f, got %.2f", expected[i], out[i].Score)
		}
	}
}

func TestMMR(t *testing.T) {
	sim := func(a, b Chunk) float64 {
		// simple deterministic similarity:
		// same prefix = high similarity
		if a.Text[0] == b.Text[0] {
			return 0.9
		}
		return 0.1
	}

	tests := map[string]struct {
		input  []Chunk
		k      int
		lambda float64
		want   []string
	}{
		"first pick is highest score": {
			input: []Chunk{
				{Text: "A", Score: 0.5},
				{Text: "B", Score: 0.9},
				{Text: "C", Score: 0.7},
			},
			k:      1,
			lambda: 0.7,
			want:   []string{"B"},
		},
		"diversity prefers different chunk": {
			input: []Chunk{
				{Text: "A1", Score: 0.9},
				{Text: "A2", Score: 0.85}, // similar to A1
				{Text: "B1", Score: 0.8},  // different
			},
			k:      2,
			lambda: 0.5,
			want:   []string{"A1", "B1"},
		},
		"high lambda favors relevance": {
			input: []Chunk{
				{Text: "A1", Score: 0.9},
				{Text: "A2", Score: 0.85}, // similar
				{Text: "B1", Score: 0.7},
			},
			k:      2,
			lambda: 0.9,
			want:   []string{"A1", "A2"},
		},
		"low lambda favors diversity": {
			input: []Chunk{
				{Text: "A1", Score: 0.9},
				{Text: "A2", Score: 0.85}, // similar
				{Text: "B1", Score: 0.7},
			},
			k:      2,
			lambda: 0.1,
			want:   []string{"A1", "B1"},
		},
		"k greater than input size": {
			input: []Chunk{
				{Text: "A", Score: 0.9},
				{Text: "B", Score: 0.8},
			},
			k:      5,
			lambda: 0.7,
			want:   []string{"A", "B"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := MMR(tt.input, tt.k, tt.lambda, sim)

			if len(got) != len(tt.want) {
				t.Fatalf("expected %d results, got %d", len(tt.want), len(got))
			}

			for i := range tt.want {
				if got[i].Text != tt.want[i] {
					t.Fatalf("at index %d: expected %s, got %s", i, tt.want[i], got[i].Text)
				}
			}
		})
	}
}
