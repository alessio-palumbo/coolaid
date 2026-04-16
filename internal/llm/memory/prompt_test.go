package memory

import (
	"coolaid/internal/store"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPrompt(t *testing.T) {
	mem := store.Memory{
		ProjectSummary: "Go CLI assistant for AI tasks",
		Topics:         []string{"memory", "LLM"},
		Preferences:    []string{"concise output"},
	}

	in := Input{
		UserInput:       "How does memory work?",
		AssistantOutput: "Memory is stored and updated asynchronously.",
	}

	got, err := buildPrompt(in, mem)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := loadGolden(t, "testdata/prompt.golden")
	assert.Equal(t, strings.TrimSpace(want), strings.TrimSpace(got))
}

func loadGolden(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	return string(b)
}
