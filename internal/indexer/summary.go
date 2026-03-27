package indexer

import (
	"ai-cli/internal/query"
	"strings"
)

const (
	readmeFragmentMaxSize = 1000
	tokenTypeMaxItems     = 20
)

type SummaryBuilder struct {
	files   []string
	symbols map[string]struct{}
	readme  string
}

func NewSummaryBuilder() *SummaryBuilder {
	return &SummaryBuilder{symbols: make(map[string]struct{})}
}

func (b *SummaryBuilder) AddFile(path string, content []byte) {
	b.files = append(b.files, path)

	if strings.HasSuffix(strings.ToLower(path), "readme.md") && b.readme == "" {
		n := min(len(content), readmeFragmentMaxSize)
		b.readme = string(content[:n])
	}

	signals := query.ExtractSignals(path, content)
	for _, s := range strings.Split(signals, "\n") {
		if s != "" {
			b.symbols[s] = struct{}{}
		}
	}
}

func (b *SummaryBuilder) Build() string {
	var sb strings.Builder

	sb.WriteString("Files:\n")
	for i, f := range b.files {
		if i >= tokenTypeMaxItems {
			break
		}
		sb.WriteString("- ")
		sb.WriteString(f)
		sb.WriteByte('\n')
	}

	sb.WriteString("\nSymbols:\n")
	i := 0
	for s := range b.symbols {
		if i >= tokenTypeMaxItems {
			break
		}
		sb.WriteString("- ")
		sb.WriteString(s)
		sb.WriteByte('\n')
		i++
	}

	if b.readme != "" {
		sb.WriteString("\nREADME:\n")
		sb.WriteString(b.readme)
	}

	return sb.String()
}
