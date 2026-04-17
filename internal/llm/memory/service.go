// Package memory provides a lightweight, project-scoped memory system
// for the agent.
//
// It extracts durable signals from user interactions (intent, preferences,
// and project context) using an LLM, then stores a compact evolving
// representation in SQLite.
package memory

import (
	"context"
	"coolaid/internal/store"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// Input represents a single interaction used for memory extraction.
//
// It combines:
//   - the user's prompt
//   - the assistant's response
type Input struct {
	UserInput       string `json:"user_input"`
	AssistantOutput string `json:"assistant_output"`
}

// extraction represents the structured delta produced by the LLM.
//
// It describes only incremental updates to memory:
//   - summary changes
//   - new topics of interest
//   - new user or project preferences
//
// Empty fields indicate no meaningful update.
type extraction struct {
	SummaryUpdate  string   `json:"summary_update"`
	TopicsAdd      []string `json:"topics_add"`
	PreferencesAdd []string `json:"preferences_add"`
}

type Store interface {
	GetMemory(ctx context.Context) (store.Memory, error)
	CommitMemoryUpdate(ctx context.Context, m store.Memory, ids []string) error

	GetMemoryQueue(ctx context.Context) ([]store.MemoryQueueItem, error)
	SaveMemoryQueue(ctx context.Context, in store.MemoryQueueItem) error
}

type LLM interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Service orchestrates project memory extraction and persistence.
//
// It:
//   - captures interaction data (assistant output)
//   - persists raw interactions for later processing
//   - retrieves current memory snapshot from storage
//   - runs LLM-based extraction to identify meaningful updates
//   - merges updates into existing memory state
//   - persists the updated memory
//
// Memory updates are not processed during Capture. Instead, they are
// persisted and processed later via FlushMemory.
type service struct {
	store Store
	llm   LLM
}

// NewService creates a new memory service instance.
//
// It wires together:
//   - persistent store (SQLite-backed memory and queue)
//   - LLM-based extractor for memory updates
//
// Memory processing is triggered explicitly via FlushMemory.
func NewService(store Store, llm LLM) *service {
	s := &service{
		store: store,
		llm:   llm,
	}
	return s
}

// Capture runs a streaming operation while capturing its output.
//
// The output is written to the provided writer AND buffered internally.
// After successful completion, the captured output is persisted to the
// memory queue for later processing.
//
// This method is non-blocking with respect to memory updates. Extraction
// and persistence of memory updates are deferred to FlushMemory.
func (s *service) Capture(w io.Writer, userPrompt string, fn func(w io.Writer) error) error {
	tw := &teeWriter{w: w}

	if err := fn(tw); err != nil {
		return err
	}

	s.storeToPersistentQueue(context.Background(), Input{
		UserInput:       userPrompt,
		AssistantOutput: tw.String(),
	})
	return nil
}

// FlushMemory processes all pending memory queue items.
//
// It:
//   - loads persisted interactions from the memory queue
//   - reconstructs inputs for memory extraction
//   - runs LLM-based extraction and updates the memory store
//   - deletes successfully processed queue items (those that contributed to memory updates)
//
// It returns the number of items successfully processed.
// Processing is best-effort: failures are logged and skipped,
// allowing items to be retried on subsequent invocations.
// Memory is updated incrementally per item; partial failures
// do not roll back earlier updates.
//
// This operation may take time depending on LLM latency.
func (s *service) FlushMemory(ctx context.Context) (int, error) {
	items, err := s.store.GetMemoryQueue(ctx)
	if err != nil {
		slog.Warn("[memory] failed to load queue", slog.String("error", err.Error()))
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	mem, err := s.store.GetMemory(ctx)
	if err != nil {
		slog.Warn("[memory] failed to load memory", slog.String("error", err.Error()))
		return 0, err
	}

	var processed int
	var itemsIDs []string
	for _, it := range items {
		if ctx.Err() != nil {
			break
		}

		in, err := fromQueueItem(it)
		if err != nil {
			slog.Warn("[memory] failed to parse persisted queue item", slog.String("error", err.Error()), slog.Any("it", it))
			continue
		}

		ex, err := s.extract(ctx, in, mem)
		if err != nil {
			slog.Warn("[memory] failed to extract memory", slog.String("id", it.ID), slog.String("error", err.Error()))
			continue // retry later
		}

		itemsIDs = append(itemsIDs, it.ID)
		mem = merge(mem, ex)
		processed++
	}

	if len(itemsIDs) == 0 {
		return 0, nil
	}

	if err := s.store.CommitMemoryUpdate(ctx, mem, itemsIDs); err != nil {
		slog.Warn("[memory] failed to commit memory", slog.Int("processed", len(itemsIDs)), slog.String("error", err.Error()))
		return 0, err
	}
	return processed, nil
}

// extract builds a prompt from the input and invokes the LLM to
// generate a structured memory update, which is then parsed.
func (s *service) extract(ctx context.Context, in Input, mem store.Memory) (extraction, error) {
	prompt, err := buildPrompt(in, mem)
	if err != nil {
		return extraction{}, err
	}

	out, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return extraction{}, err
	}

	return parseExtraction(out)
}

// storeToPersistentQueue serializes and stores an input for later
// memory processing. Failures are logged and ignored.
func (s *service) storeToPersistentQueue(ctx context.Context, in Input) {
	it, err := toQueueItem(in)
	if err != nil {
		slog.Warn("[memory] failed to convert Input", slog.String("error", err.Error()))
	}
	if err := s.store.SaveMemoryQueue(ctx, it); err != nil {
		slog.Warn("[memory] failed to store queue", slog.String("error", err.Error()))
	}
}

func parseExtraction(s string) (extraction, error) {
	var ex extraction

	jsonStr := extractJSON(s)
	if jsonStr == "" {
		return ex, errors.New("no JSON found in response")
	}

	err := json.Unmarshal([]byte(jsonStr), &ex)
	return ex, err
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")

	if start == -1 || end == -1 || end < start {
		return ""
	}

	return s[start : end+1]
}

func toMemoryString(m store.Memory) string {
	if m.ProjectSummary == "" && len(m.Topics) == 0 && len(m.Preferences) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("PROJECT SUMMARY:\n")
	b.WriteString(m.ProjectSummary)
	b.WriteString("\n\n")

	b.WriteString("TOPICS:\n")
	for _, t := range m.Topics {
		b.WriteString("- ")
		b.WriteString(t)
		b.WriteString("\n")
	}

	b.WriteString("\nPREFERENCES:\n")
	for _, p := range m.Preferences {
		b.WriteString("- ")
		b.WriteString(p)
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func toQueueItem(in Input) (store.MemoryQueueItem, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return store.MemoryQueueItem{}, err
	}
	return store.MemoryQueueItem{
		ID:      uuid.NewString(),
		Payload: b,
	}, nil
}

func fromQueueItem(item store.MemoryQueueItem) (Input, error) {
	var in Input
	err := json.Unmarshal(item.Payload, &in)
	return in, err
}
