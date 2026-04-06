package retrieval

// Chunk represents a unit of context passed to the LLM.
// It contains the text content along with its source and optional score.
type Chunk struct {
	Text   string
	Source string
	Score  float64
}
