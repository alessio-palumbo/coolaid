package engine

import (
	"context"
	"coolaid/internal/prompts"
	"coolaid/internal/retrieval"
	"coolaid/internal/store"
	"errors"
	"io"
)

// ErrTargetFileRequired is returned when a task requires a target file:[fn].
var ErrTargetFileRequired = errors.New("target file required")

// Target represents a resolved code location used during engine execution.
//
// It is derived from the external API Target and may include additional
// processing or normalization for retrieval and prompt construction.
type Target struct {
	File      string
	Function  string
	StartLine int
	EndLine   int
}

func (t Target) validate() error {
	if t.File == "" {
		return ErrTargetFileRequired
	}
	return nil
}

// TaskConfig holds internal configuration derived from TaskOptions.
type TaskConfig struct {
	Retrieval RetrievalOptions
	Prompt    PromptTaskOptions
	Web       WebOptions
	Handlers  []ResultHandler
}

// ResultHandler processes final output of a task.
type ResultHandler interface {
	Handle(ctx context.Context, output string) error
}

// RetrievalOptions controls how semantic search is performed.
type RetrievalOptions struct {
	// K is the number of top results to retrieve.
	K int

	// UseMMR enables Max Marginal Relevance to improve diversity
	// among retrieved results.
	UseMMR bool
}

// PromptTaskOptions controls how prompts are constructed for the LLM.
type PromptTaskOptions struct {
	// SystemOverride replaces the default system prompt used in templates.
	SystemOverride string

	// StructuredOutput enables structured (machine-readable) responses
	// when supported by the underlying prompt/template.
	StructuredOutput bool
}

// WebOptions controls optional web search augmentation for tasks.
type WebOptions struct {
	SearchLimit int
}

// task represents an internal execution unit for LLM-based operations.
//
// It encapsulates all inputs required to build a prompt (target, template,
// and prompt text), configuration options (retrieval, formatting, etc.),
// and optional post-processing handlers that operate on the final LLM output.
//
// If no handlers are provided, the task runs in streaming mode.
// If handlers are present, the full response is generated and passed to them.
type task struct {
	UserPrompt string
	Template   prompts.Template
	Target     Target
	TargetBody string
	Summary    string
	config     *TaskConfig
}

// newTask constructs an internal task and applies all TaskOptions,
// producing a parsed task configuration ready for execution.
func newTask(userPrompt string, target Target, tmpl prompts.Template, cfg *TaskConfig) task {
	return task{
		UserPrompt: userPrompt,
		Target:     target,
		Template:   tmpl,
		config:     cfg,
	}
}

// buildPrompt renders the final LLM prompt using the task template,
// memory state, optional retrieval chunks, and any derived task inputs
// such as repository summary or target body.
// It returns a fully formatted prompt ready for execution.
func (t task) buildPrompt(memory store.Memory, chunks ...retrieval.Chunk) (string, error) {
	cfg := &prompts.Config{
		Template:       t.Template,
		Memory:         memory,
		SystemOverride: t.config.Prompt.SystemOverride,
		Structured:     t.config.Prompt.StructuredOutput,
		Summary:        t.Summary,
	}
	if t.TargetBody != "" {
		cfg = cfg.WithTarget(t.Target.File, t.Target.Function, t.TargetBody)
	}
	return prompts.Render(cfg, t.UserPrompt, chunks...)
}

// execute runs the task using streaming mode by default.
//
// If result handlers are configured, it switches to buffered generation
// and passes the final output to each handler.
func (t task) execute(ctx context.Context, llm LLM, w io.Writer, prompt string) error {
	if len(t.config.Handlers) > 0 {
		out, err := llm.Generate(ctx, prompt)
		if err != nil {
			return err
		}
		return t.runHandlers(ctx, out)
	}

	return llm.GenerateStream(ctx, prompt, w)
}

// runHandlers invokes all configured result handlers in order,
// passing each the final generated output.
func (t task) runHandlers(ctx context.Context, out string) error {
	for _, h := range t.config.Handlers {
		if err := h.Handle(ctx, out); err != nil {
			return err
		}
	}
	return nil
}
