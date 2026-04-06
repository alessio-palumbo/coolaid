package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTextChunker_Chunk(t *testing.T) {
	tests := []struct {
		name             string
		text             string
		overrideMaxChars int
		want             []string
	}{
		{
			name: "empty input",
			text: "",
		},
		{
			name: "single paragraph",
			text: "This is a single paragraph.",
			want: []string{"This is a single paragraph."},
		},
		{
			name:             "multiple paragraphs with no wrapping",
			text:             "Paragraph 1. This is the first paragraph.\n\nParagraph 2. This is the second paragraph.",
			overrideMaxChars: 40,
			want:             []string{"Paragraph 1. This is the first paragraph.", "Paragraph 2. This is the second paragraph."},
		},
		{
			name: "multiple paragraphs with wrapping",
			text: "Paragraph 1. This is the first paragraph.\n\nParagraph 2. This is the second paragraph.",
			want: []string{"Paragraph 1. This is the first paragraph.", "Paragraph 2. This is the second paragraph."},
		},
		{
			name:             "paragraphs with varying lengths",
			text:             "Short paragraph.\n\nLonger paragraph that exceeds chunk size.\n\nAnother short paragraph.",
			overrideMaxChars: 40,
			want:             []string{"Short paragraph.\n\nLonger paragraph that exceeds chunk size.", "Another short paragraph."},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.overrideMaxChars > 0 {
				chunkMaxChars = test.overrideMaxChars
			}
			chunker := &TextChunker{}
			got := chunker.Chunk(test.text)
			assert.Equal(t, test.want, got)
		})
	}
}
