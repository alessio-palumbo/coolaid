package indexer

import (
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultChunkCharacters = 400
	minChunkSize           = 50
	chunkPathPrefix        = "file: "
)

type Chunk struct {
	Text      string
	StartLine int
	EndLine   int
}

func ChunkFile(path string, content []byte) []Chunk {
	switch filepath.Ext(path) {
	case ".go":
		return ChunkGo(path, content)
	default:
		return ChunkText(path, content)
	}
}

// ChunkText splits arbitrary text into fixed-size chunks.
// The file path and line numbers are included to preserve context for embeddings.
func ChunkText(path string, content []byte) []Chunk {
	if len(content) == 0 {
		return nil
	}

	startByte := 0
	startLine := 1
	charCount := 0
	line := 1

	// Pre-allocate approximate chunk capacity
	chunks := make([]Chunk, 0, len(content)/defaultChunkCharacters+1)

	for i := range len(content) {
		charCount++
		if content[i] == '\n' {
			line++
		}

		if charCount >= defaultChunkCharacters {
			endByte := i + 1
			// trim trailing newline
			if endByte < len(content) && content[endByte-1] == '\n' {
				endByte--
			}

			chunks = append(chunks, Chunk{
				StartLine: startLine,
				EndLine:   line,
				Text:      formatChunk(path, startLine, line, content[startByte:endByte]),
			})

			startByte = endByte
			startLine = line
			charCount = 0
		}
	}

	if startByte < len(content) {
		if charCount >= minChunkSize {
			chunks = append(chunks, Chunk{
				StartLine: startLine,
				EndLine:   line,
				Text:      formatChunk(path, startLine, line, content[startByte:]),
			})
		}
	}

	return chunks
}

func formatChunk(path string, startLine, endLine int, body []byte) string {
	const lineNumbersApproxLen = 32
	totalSize := len(chunkPathPrefix) + len(path) + lineNumbersApproxLen + len(body)

	var sb strings.Builder
	sb.Grow(totalSize)

	sb.WriteString(chunkPathPrefix)
	sb.WriteString(path)

	sb.WriteString(" (lines ")
	// preallocate slice for line numbers
	b := make([]byte, 0, lineNumbersApproxLen)
	b = strconv.AppendInt(b, int64(startLine), 10)
	b = append(b, '-')
	b = strconv.AppendInt(b, int64(endLine), 10)
	sb.Write(b)

	sb.WriteString(")\n\n")
	sb.Write(body)

	return sb.String()
}
