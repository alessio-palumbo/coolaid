package vector

import (
	"math"
	"testing"
)

func Test_cosine(t *testing.T) {
	a := normalize([]float64{1, 0})
	b := normalize([]float64{1, 0})

	score := cosine(a, b)
	if score < 0.99 {
		t.Fatalf("expected vectors to be similar")
	}
}

func Test_normalize(t *testing.T) {
	v := []float64{3, 4}
	n := normalize(v)
	length := math.Sqrt(n[0]*n[0] + n[1]*n[1])

	if math.Abs(length-1.0) > 0.0001 {
		t.Fatalf("vector not normalized, length=%f", length)
	}
}
