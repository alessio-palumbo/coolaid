package store

import "math"

// cosine returns the cosine similarity between two normalized vectors.
// If vectors are unit length, this is equivalent to their dot product.
func cosine(a, b []float64) float64 {
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}

// normalize scales a vector to unit length so it can be compared
// using cosine similarity.
func normalize(v []float64) []float64 {
	var norm float64
	for _, x := range v {
		norm += x * x
	}

	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}

	for i := range v {
		v[i] /= norm
	}
	return v
}
