// Package memory provides a lightweight, project-scoped memory system
// for the agent.
//
// It extracts durable signals from user interactions (intent, preferences,
// and project context) using an LLM, then stores a compact evolving
// representation in SQLite.
//
// Memory updates are performed asynchronously via a background queue
// and are best-effort (non-blocking, non-critical path).
package memory

import (
	"context"
	"coolaid/internal/store"
	"encoding/json"
	"io"
	"strings"
)

// Input represents a single interaction used for memory extraction.
//
// It combines:
//   - the user's prompt
//   - the assistant's response
//   - any retrieved RAG context
//   - the current persisted memory snapshot (as a string)
type Input struct {
	UserInput        string
	AssistantOutput  string
	RetrievedContext string
	CurrentMemory    string
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
	SaveMemory(ctx context.Context, m store.Memory) error
}

type LLM interface {
	Generate(prompt string) (string, error)
}

// Service orchestrates project memory extraction and persistence.
//
// It:
//   - collects interaction data (user input, assistant output, context)
//   - retrieves current memory snapshot from storage
//   - runs LLM-based extraction to identify meaningful updates
//   - merges updates into existing memory state
//   - persists the updated memory
//
// Memory updates are queued asynchronously and processed in the background
// to avoid blocking CLI execution.
type service struct {
	store Store
	llm   LLM
	queue *queue
}

// NewService creates a new memory service instance.
//
// It wires together:
//   - persistent store (SQLite-backed memory)
//   - LLM-based extractor for memory updates
//   - internal async queue for non-blocking processing
//
// The returned service immediately starts a background worker
// to process memory updates asynchronously.
func NewService(store Store, llm LLM) *service {
	s := &service{
		store: store,
		llm:   llm,
	}
	s.queue = newQueue(s)
	return s
}

// Capture runs a streaming operation while capturing its output.
//
// The output is written to the provided writer AND buffered internally.
// After successful completion, the captured output is attached to the
// memory input and enqueued for asynchronous extraction.
//
// This is the primary integration point between LLM streaming and memory
// persistence.
func (s *service) Capture(w io.Writer, in Input, fn func(w io.Writer) error) error {
	tw := &teeWriter{w: w}

	if err := fn(tw); err != nil {
		return err
	}
	if s.queue == nil {
		return nil
	}

	in.AssistantOutput = tw.String()
	s.queue.Enqueue(in)

	return nil
}

// Close gracefully shuts down the memory queue and waits for
// any in-flight memory updates to complete.
func (s *service) Close(ctx context.Context) {
	if s.queue != nil {
		s.queue.close(ctx)
	}
}

// ExtractAndUpdate performs a full memory update cycle:
//
//  1. Loads current project memory from store
//  2. Builds enriched input context
//  3. Uses LLM to extract structured memory updates
//  4. Merges updates into existing memory
//  5. Persists updated memory back to store
//
// Errors are propagated; memory updates are not retried automatically.
func (s *service) extractAndUpdate(ctx context.Context, in Input) error {
	current, err := s.store.GetMemory(ctx)
	if err != nil {
		return err
	}

	in.CurrentMemory = toMemoryString(current)
	ex, err := s.extract(ctx, in)
	if err != nil {
		return err
	}

	merged := merge(current, ex)
	return s.store.SaveMemory(ctx, merged)
}

func (s *service) extract(ctx context.Context, in Input) (extraction, error) {
	prompt, err := buildPrompt(in)
	if err != nil {
		return extraction{}, err
	}

	out, err := s.llm.Generate(prompt)
	if err != nil {
		return extraction{}, err
	}

	return parseExtraction(out)
}

func parseExtraction(s string) (extraction, error) {
	var ex extraction
	err := json.Unmarshal([]byte(s), &ex)
	return ex, err
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
