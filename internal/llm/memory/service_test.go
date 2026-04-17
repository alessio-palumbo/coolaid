package memory

import (
	"context"
	"coolaid/internal/store"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlushMemory(t *testing.T) {
	testCases := map[string]struct {
		initialQueue []store.MemoryQueueItem
		store        *fakeStore
		llm          *fakeLLM
		expectErr    bool
		expected     store.Memory
		expectedProc int
	}{
		"adds topic": {
			initialQueue: []store.MemoryQueueItem{
				{ID: "1", Payload: []byte(`{"UserInput":"test"}`)},
			},
			store: &fakeStore{
				mem: store.Memory{},
			},
			llm: &fakeLLM{
				resp: `{"topics_add":["rag"]}`,
			},
			expected: store.Memory{
				Topics: []string{"rag"},
			},
			expectedProc: 1,
		},
		"adds preference and summary": {
			initialQueue: []store.MemoryQueueItem{
				{ID: "1", Payload: []byte(`{"UserInput":"use short answers"}`)},
			},
			store: &fakeStore{},
			llm: &fakeLLM{
				resp: `{"preferences_add":["concise"],"summary_update":"user prefers concise responses"}`,
			},
			expected: store.Memory{
				ProjectSummary: "user prefers concise responses",
				Preferences:    []string{"concise"},
			},
			expectedProc: 1,
		},
		"merges existing memory": {
			initialQueue: []store.MemoryQueueItem{
				{ID: "1", Payload: []byte(`{"UserInput":"more rag work"}`)},
			},
			store: &fakeStore{
				mem: store.Memory{
					Topics: []string{"rag"},
				},
			},
			llm: &fakeLLM{
				resp: `{"topics_add":["vector search"]}`,
			},
			expected: store.Memory{
				Topics: []string{"rag", "vector search"},
			},
			expectedProc: 1,
		},
		"invalid json returns no change": {
			initialQueue: []store.MemoryQueueItem{
				{ID: "1", Payload: []byte(`NOT_JSON`)},
			},
			store: &fakeStore{
				mem: store.Memory{
					Topics: []string{"existing"},
				},
			},
			llm: &fakeLLM{
				resp: `NOT_JSON`,
			},
			expected: store.Memory{
				Topics: []string{"existing"},
			},
			expectedProc: 0,
		},
		"empty extraction no-op": {
			initialQueue: []store.MemoryQueueItem{
				{ID: "1", Payload: []byte(`{"UserInput":"test"}`)},
			},
			store: &fakeStore{
				mem: store.Memory{
					Topics: []string{"existing"},
				},
			},
			llm: &fakeLLM{
				resp: `{}`,
			},
			expected: store.Memory{
				Topics: []string{"existing"},
			},
			expectedProc: 1,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.store.memQueue = tc.initialQueue

			s := NewService(tc.store, tc.llm)

			processed, err := s.FlushMemory(context.Background())
			if err != nil {
				if !tc.expectErr {
					t.Fatal(err)
				}
			}

			assert.Equal(t, tc.expectedProc, processed)

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

func (f fakeLLM) Generate(ctx context.Context, prompt string) (string, error) {
	return f.resp, nil
}

type fakeStore struct {
	mem      store.Memory
	memQueue []store.MemoryQueueItem
}

func (f *fakeStore) GetMemory(ctx context.Context) (store.Memory, error) {
	return f.mem, nil
}
func (f *fakeStore) CommitMemoryUpdate(ctx context.Context, m store.Memory, ids []string) error {
	f.mem = m
	return nil
}

func (f *fakeStore) GetMemoryQueue(ctx context.Context) ([]store.MemoryQueueItem, error) {
	return f.memQueue, nil
}

func (f *fakeStore) SaveMemoryQueue(ctx context.Context, in store.MemoryQueueItem) error {
	f.memQueue = append(f.memQueue, in)
	return nil
}
