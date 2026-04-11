package retrieval

import "container/heap"

// ChunkHeap keeps track of the top scoring Chunks.
type ChunkHeap []Chunk

func (h ChunkHeap) Len() int           { return len(h) }
func (h ChunkHeap) Less(i, j int) bool { return h[i].Score < h[j].Score }
func (h ChunkHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *ChunkHeap) Push(x any) {
	*h = append(*h, x.(Chunk))
}

func (h *ChunkHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func (h *ChunkHeap) DrainDesc() []Chunk {
	n := h.Len()
	out := make([]Chunk, n)

	// pop min → fill from end → descending result
	// use heap package to maintain heap behaviour, e.g. ordering.
	for i := n - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(Chunk)
	}
	return out
}
