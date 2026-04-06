package web

import (
	"strings"
)

var chunkMaxChars = 3500

// TextChunker splits plain text into smaller chunks suitable for LLM input.
// It groups content by paragraphs and enforces a maximum chunk size.
type TextChunker struct{}

func NewTextChunker() *TextChunker {
	return &TextChunker{}
}

// Chunk splits the input text into multiple segments based on paragraph
// boundaries while respecting the configured maximum size.
func (c *TextChunker) Chunk(text string) []string {
	var chunks []string
	var current strings.Builder
	const sep = "\n\n"

	for p := range strings.SplitSeq(strings.TrimSpace(text), sep) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if current.Len() > 0 && current.Len() > chunkMaxChars {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString(sep)
		}
		current.WriteString(p)
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}
