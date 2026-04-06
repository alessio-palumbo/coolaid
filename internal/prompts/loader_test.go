package prompts

import (
	"ai-cli/internal/retrieval"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var update = flag.Bool("update", false, "update golden files")

func TestRender(t *testing.T) {
	tests := map[string]struct {
		cfg     *Config
		prompt  string
		context []retrieval.Chunk
		golden  string
	}{
		"explain_file_structured": {
			cfg: (&Config{
				Template:   TemplateExplain,
				Structured: true,
			}).WithTarget("main.go", ""),
			prompt: "package main\n\nfunc main() {}",
			context: []retrieval.Chunk{
				{Text: "func helper() {}", Source: "/example/file.go", Score: 0.879},
			},
			golden: "explain_file_structured.golden",
		},

		"explain_function": {
			cfg: (&Config{
				Template: TemplateExplain,
			}).WithTarget("main.go", "main"),
			prompt: "func main() {}",
			context: []retrieval.Chunk{
				{Text: "func helper() {}", Source: "/example/file.go"},
			},
			golden: "explain_function.golden",
		},

		"summarize_basic": {
			cfg: &Config{
				Template: TemplateSummarize,
			},
			prompt:  "package main\n\nfunc main() {}",
			context: nil,
			golden:  "summarize_basic.golden",
		},

		"with_summary": {
			cfg: &Config{
				Template: TemplateSummarize,
				Summary:  "This repository handles user management.",
			},
			prompt: "package main\n\nfunc main() {}",
			context: []retrieval.Chunk{
				{Text: "user repository code", Source: "/example/file.go"},
			},
			golden: "with_summary.golden",
		},

		"system_override": {
			cfg: &Config{
				Template:       TemplateChat,
				SystemOverride: "You are a strict reviewer.",
			},
			prompt:  "Review this code",
			context: nil,
			golden:  "system_override.golden",
		},

		"no_context": {
			cfg: &Config{
				Template: TemplateQuery,
			},
			prompt:  "What does this do?",
			context: nil,
			golden:  "no_context.golden",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Render(tt.cfg, tt.prompt, tt.context...)
			assert.NoError(t, err)

			path := filepath.Join("testdata", tt.golden)

			if *update {
				err := os.WriteFile(path, []byte(got), 0644)
				assert.NoError(t, err)
				return
			}

			want := loadGolden(t, path)

			// Full comparison
			assert.Equal(t, normalize(want), normalize(got))

			// 🔍 Structural sanity checks (important)
			assert.Contains(t, got, "USER REQUEST:")

			if tt.cfg.Summary != "" {
				assert.Contains(t, got, "REPOSITORY OVERVIEW:")
			}

			// Ensure ordering: CONTEXT comes before USER REQUEST (if present)
			ctxIdx := strings.Index(got, "CONTEXT:")
			reqIdx := strings.Index(got, "USER REQUEST:")

			if ctxIdx != -1 && reqIdx != -1 {
				assert.Less(t, ctxIdx, reqIdx, "context should appear before user request")
			}
		})
	}
}

func loadGolden(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	return string(b)
}

func normalize(s string) string {
	return strings.TrimSpace(s)
}
