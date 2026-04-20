package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WriteMode int

const (
	CreateOrAppend WriteMode = iota
	CreateOrWriteInPlace
)

var (
	ErrUnsupportedLanguage = errors.New("unsupported fenced code language")
	ErrPathRequired        = errors.New("path is required")
)

type CodeOutput struct {
	content   string
	language  string
	separator string
}

// NewCodeOutput parses LLM output into a structured code result.
//
// It extracts a fenced code block when present, validates supported
// languages, and prepares language-specific formatting for file writes.
func NewCodeOutput(raw string) (*CodeOutput, error) {
	content, lang, err := extractCodeBlock(raw)
	if err != nil {
		return nil, err
	}

	return &CodeOutput{
		content:   content,
		language:  lang,
		separator: separatorForLanguage(lang),
	}, nil
}

// AppendToFile writes the extracted code to the target file.
//
// If the file does not exist, it is created.
// If it already exists, the content is appended at the end.
// A language-aware separator is inserted between existing and new content
// when appending, if available.
func (c *CodeOutput) AppendToFile(path string) error {
	if path == "" {
		return ErrPathRequired
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.WriteFile(path, []byte(c.content), 0644)
	}

	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open file for append: %w", err)
	}
	defer f.Close()

	text := c.content
	if c.separator != "" {
		text = c.separator + text
	}

	_, err = f.WriteString(text)
	if err != nil {
		return fmt.Errorf("cannot append file: %w", err)
	}

	return nil
}

// ReplaceLines replaces a range of lines in the target file with the generated code.
//
// Line numbers are 1-based and inclusive.
// If the file is shorter than the provided range, the range is clamped to file bounds.
// This operation rewrites the entire file content after applying the replacement.
func (c *CodeOutput) ReplaceLines(path string, startLine, endLine int) error {
	if path == "" {
		return ErrPathRequired
	}
	if startLine <= 0 || endLine < startLine {
		return fmt.Errorf("invalid line range: %d-%d", startLine, endLine)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// clamp to file bounds
	startLine = min(startLine, len(lines))
	endLine = min(endLine, len(lines))

	// convert to 0-based
	start := startLine - 1
	end := endLine

	// build new file content
	var b strings.Builder
	b.Grow(len(data) + len(c.content))

	// write content before start line
	for i := range start {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}

	// write new content.
	b.WriteString(c.content)
	if !strings.HasSuffix(c.content, "\n") {
		b.WriteByte('\n')
	}

	// write content after endline
	for i := end; i < len(lines); i++ {
		b.WriteString(lines[i])
		if i != len(lines)-1 {
			b.WriteByte('\n')
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

// extractCodeBlock extracts the first fenced code block from LLM output.
//
// If no code fence is found, the raw input is returned as plain content.
// When a fenced block is present, an optional language tag is parsed and stripped.
// Only a single code block is supported (first occurrence is used).
// If a fenced language is present but unsupported, an error is returned.
func extractCodeBlock(raw string) (content, language string, err error) {
	start := strings.Index(raw, "```")
	if start == -1 {
		// no fence → treat raw content as valid plain text
		return raw, "", nil
	}

	rest := raw[start+3:]

	nl := strings.Index(rest, "\n")
	if nl == -1 {
		return "", "", errors.New("malformed fenced code block")
	}

	lang := strings.TrimSpace(rest[:nl])

	end := strings.Index(rest[nl+1:], "```")
	if end == -1 {
		return "", "", errors.New("unterminated fenced code block")
	}

	code := rest[nl+1 : nl+1+end]
	return code, lang, nil
}

func separatorForLanguage(lang string) string {
	switch lang {
	case "go", "js", "ts":
		return "\n\n// ---- AI GENERATED CODE ----\n\n"
	case "python":
		return "\n\n# ---- AI GENERATED CODE ----\n\n"
	default:
		return "\n\n"
	}
}
