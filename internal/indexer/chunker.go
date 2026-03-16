package indexer

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	defaultChunkCharacters = 400
	minChunkSize           = 50
	chunkPrefix            = "file: "
)

func ChunkFile(path string, content string) []string {
	switch filepath.Ext(path) {
	case ".go":
		return ChunkGo(path, content)
	default:
		return ChunkText(path, content)
	}
}

// ChunkText splits arbitrary text into fixed-size chunks.
// The file path is included to preserve context for embeddings.
func ChunkText(path string, content string) []string {
	var chunks []string
	runes := []rune(content)

	for i := 0; i < len(runes); i += defaultChunkCharacters {
		end := min(i+defaultChunkCharacters, len(runes))
		if body := string(runes[i:end]); utf8.RuneCountInString(body) > minChunkSize {
			chunks = append(chunks, formatChunk(path, body))
		}
	}
	return chunks
}

func formatChunk(path string, text ...string) string {
	var sb strings.Builder
	totalSize := len(chunkPrefix) + len(path) + 1
	for _, t := range text {
		totalSize += len(t) + 1
	}
	sb.Grow(totalSize)

	sb.WriteString(chunkPrefix)
	sb.WriteString(path)
	sb.WriteByte('\n')

	for _, t := range text {
		sb.WriteByte('\n')
		sb.WriteString(t)
	}
	return sb.String()
}
