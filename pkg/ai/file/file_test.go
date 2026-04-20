package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendToFile(t *testing.T) {
	testCases := map[string]struct {
		initial  string
		content  string
		expected string
	}{
		"creates new file": {
			initial:  "",
			content:  "line1",
			expected: "line1",
		},
		"appends with separator": {
			initial:  "existing",
			content:  "```go\nnew content\n```",
			expected: "existing\n\n// ---- AI GENERATED CODE ----\n\nnew content\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file.txt")

			if tc.initial != "" {
				require.NoError(t, os.WriteFile(path, []byte(tc.initial), 0644))
			}

			out, err := NewCodeOutput(tc.content)
			require.NoError(t, err)

			err = out.AppendToFile(path)
			require.NoError(t, err)

			got, err := os.ReadFile(path)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, string(got))
		})
	}
}
func TestReplaceLines(t *testing.T) {
	testCases := map[string]struct {
		initial  string
		start    int
		end      int
		content  string
		expected string
	}{
		"replaces middle lines": {
			initial:  "a\nb\nc\nd",
			start:    2,
			end:      3,
			content:  "X",
			expected: "a\nX\nd",
		},
		"replaces single line": {
			initial:  "a\nb\nc",
			start:    2,
			end:      2,
			content:  "B",
			expected: "a\nB\nc",
		},
		"out of bounds clamps safely": {
			initial:  "a\nb",
			start:    2,
			end:      10,
			content:  "Z",
			expected: "a\nZ\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file.txt")

			require.NoError(t, os.WriteFile(path, []byte(tc.initial), 0644))

			out, err := NewCodeOutput(tc.content)
			require.NoError(t, err)

			err = out.ReplaceLines(path, tc.start, tc.end)
			require.NoError(t, err)

			got, err := os.ReadFile(path)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, string(got))
		})
	}
}

func Test_extractCodeBlock(t *testing.T) {
	testCases := map[string]struct {
		input        string
		expected     string
		expectedLang string
		expectError  bool
	}{
		"no code block returns raw text": {
			input:        "just plain text",
			expected:     "just plain text",
			expectedLang: "",
		},
		"simple go block": {
			input:        "```go\nfmt.Println(\"hi\")\n```",
			expected:     "fmt.Println(\"hi\")\n",
			expectedLang: "go",
		},
		"block with language tag removed": {
			input:        "```python\nprint('hi')\n```",
			expected:     "print('hi')\n",
			expectedLang: "python",
		},
		"no language tag": {
			input:        "```\ncode only\n```",
			expected:     "code only\n",
			expectedLang: "",
		},
		"multiple fences uses first only": {
			input:        "```go\na\n```\n```go\nb\n```",
			expected:     "a\n",
			expectedLang: "go",
		},
		"unterminated fence returns error": {
			input:        "```go\nincomplete",
			expected:     "",
			expectedLang: "",
			expectError:  true,
		},
		"unknown language treated as plain block (no error)": {
			input:        "```rust\nlet x = 1;\n```",
			expected:     "let x = 1;\n",
			expectedLang: "rust",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, lang, err := extractCodeBlock(tc.input)

			require.Equal(t, err != nil, tc.expectError)
			assert.Equal(t, tc.expected, got)
			assert.Equal(t, tc.expectedLang, lang)
		})
	}
}
