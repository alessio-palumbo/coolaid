package memory

import (
	"context"
	"coolaid/internal/store"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAndUpdate(t *testing.T) {
	testCases := map[string]struct {
		input     Input
		store     *fakeStore
		llm       *fakeLLM
		expectErr bool
		expected  store.Memory
	}{
		"adds topic": {
			input: Input{UserInput: "test"},
			store: &fakeStore{mem: store.Memory{}},
			llm: &fakeLLM{
				resp: `{"topics_add":["rag"]}`,
			},
			expected: store.Memory{
				Topics: []string{"rag"},
			},
		},

		"adds preference and summary": {
			input: Input{UserInput: "use short answers"},
			store: &fakeStore{mem: store.Memory{}},
			llm: &fakeLLM{
				resp: `{"preferences_add":["concise"],"summary_update":"user prefers concise responses"}`,
			},
			expected: store.Memory{
				ProjectSummary: "user prefers concise responses",
				Preferences:    []string{"concise"},
			},
		},

		"merges existing memory": {
			input: Input{UserInput: "more rag work"},
			store: &fakeStore{mem: store.Memory{
				Topics: []string{"rag"},
			}},
			llm: &fakeLLM{
				resp: `{"topics_add":["vector search"]}`,
			},
			expected: store.Memory{
				Topics: []string{"rag", "vector search"},
			},
		},

		"invalid json returns no change": {
			input: Input{UserInput: "test"},
			store: &fakeStore{mem: store.Memory{
				Topics: []string{"existing"},
			}},
			llm: &fakeLLM{
				resp: `NOT_JSON`,
			},
			expectErr: true,
			expected: store.Memory{
				Topics: []string{"existing"},
			},
		},

		"empty extraction no-op": {
			input: Input{UserInput: "test"},
			store: &fakeStore{mem: store.Memory{
				Topics: []string{"existing"},
			}},
			llm: &fakeLLM{
				resp: `{}`,
			},
			expected: store.Memory{
				Topics: []string{"existing"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := NewService(tc.store, tc.llm)
			if err := s.extractAndUpdate(context.Background(), tc.input); err != nil {
				if !tc.expectErr {
					t.Fatal(err)
				}
			}

			got, _ := tc.store.GetMemory(context.Background())
			assert.Equal(t, tc.expected.ProjectSummary, got.ProjectSummary)
			assert.Equal(t, tc.expected.Topics, got.Topics)
			assert.Equal(t, tc.expected.Preferences, got.Preferences)
		})
	}
}

type fakeLLM struct {
	resp string
}

func (f fakeLLM) Generate(prompt string) (string, error) {
	return f.resp, nil
}

type fakeStore struct {
	mem store.Memory
}

func (f *fakeStore) GetMemory(ctx context.Context) (store.Memory, error) {
	return f.mem, nil
}
func (f *fakeStore) SaveMemory(ctx context.Context, m store.Memory) error {
	f.mem = m
	return nil
}
