package web

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Extractor defines a component that converts raw HTML into
// cleaned, readable text suitable for downstream processing.
type Extractor interface {
	Extract(html string) (string, error)
}

// SimpleExtractor is a lightweight HTML-to-text extractor.
// It removes common non-content elements (scripts, nav, etc.)
// and returns the visible body text.
type SimpleExtractor struct{}

func NewSimpleExtractor() *SimpleExtractor {
	return &SimpleExtractor{}
}

// Extract parses the provided HTML and returns cleaned text content.
// It strips common non-content elements and normalizes whitespace.
func (e *SimpleExtractor) Extract(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	// Remove junk
	doc.Find("script, style, nav, footer, header, iframe, noscript, svg, img").Remove()

	text := doc.Find("body").Text()

	return clean(text), nil
}

// clean normalizes extracted text by collapsing whitespace
// and removing excessive newlines for better chunking.
func clean(s string) string {
	// Normalize all variations of "empty lines" to a standard \n\n
	// This ensures the split actually catches all paragraph breaks
	s = strings.ReplaceAll(s, "\r\n", "\n")

	// Split by single newline to find all potential lines
	lines := strings.Split(s, "\n")
	var cleaned []string
	var currentParagraph []string

	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed != "" {
			currentParagraph = append(currentParagraph, trimmed)
		} else if len(currentParagraph) > 0 {
			// We hit an empty line, join the accumulated lines into one paragraph
			cleaned = append(cleaned, strings.Join(currentParagraph, " "))
			currentParagraph = nil
		}
	}

	// Catch the last paragraph
	if len(currentParagraph) > 0 {
		cleaned = append(cleaned, strings.Join(currentParagraph, " "))
	}

	return strings.Join(cleaned, "\n\n")
}
