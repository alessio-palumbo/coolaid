package indexer

import (
	"go/ast"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultChunkCharacters = 400
	minChunkSize           = 50

	astKindMethod   = "method"
	astKindFunction = "function"
)

type Chunk struct {
	StartLine int
	EndLine   int
	Text      string
	Symbol    string
	Kind      string
}

func NewChunk(startLine, endLine int, fn *ast.FuncDecl, text string) Chunk {
	c := Chunk{Text: text, StartLine: startLine, EndLine: endLine}
	if fn != nil {
		c.Symbol = fn.Name.Name
		if fn.Recv != nil {
			c.Kind = astKindMethod
		} else {
			c.Kind = astKindFunction
		}
	}
	return c
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

			chunks = append(chunks, NewChunk(
				startLine, line, nil,
				formatChunk(path, startLine, line, nil, content[startByte:endByte]),
			))

			startByte = endByte
			startLine = line
			charCount = 0
		}
	}

	if startByte < len(content) {
		if charCount >= minChunkSize {
			chunks = append(chunks, NewChunk(
				startLine, line, nil,
				formatChunk(path, startLine, line, nil, content[startByte:]),
			))
		}
	}

	return chunks
}

func formatChunk(path string, startLine, endLine int, fn *ast.FuncDecl, body []byte) string {
	const lineNumbersApproxLen = 16
	const contextApproxLen = 60
	totalSize := contextApproxLen + len(path) + lineNumbersApproxLen + len(body)

	var sb strings.Builder
	sb.Grow(totalSize)

	sb.WriteString("File: ")
	sb.WriteString(path)
	sb.WriteString("\n")

	if fn != nil {
		sb.WriteString("Symbol: ")
		sb.WriteString(fn.Name.Name)
		sb.WriteString("\n")
		sb.WriteString("Kind: ")
		if fn.Recv != nil {
			sb.WriteString(astKindMethod)
		} else {
			sb.WriteString(astKindFunction)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Lines: ")
	// preallocate slice for line numbers
	b := make([]byte, 0, lineNumbersApproxLen)
	b = strconv.AppendInt(b, int64(startLine), 10)
	b = append(b, '-')
	b = strconv.AppendInt(b, int64(endLine), 10)
	sb.Write(b)

	sb.WriteString("\n\n")
	sb.Write(body)

	return sb.String()
}
