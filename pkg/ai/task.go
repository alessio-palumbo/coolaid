package ai

import (
	"context"
	"coolaid/internal/core/engine"
	"coolaid/internal/prompts"
	"errors"
	"path/filepath"
)

var (
	// ErrEmptyPrompt is returned when a task requires a non-empty prompt.
	ErrEmptyPrompt = errors.New("prompt required")
)

// TaskResult represents the outcome of a task execution.
type TaskResult struct {
	Status TaskStatus
}

// TaskStatus provides additional information about the task outcome.
type TaskStatus struct {
	// NoResults indicates that no relevant results were found for the query.
	// This is not considered an error condition.
	NoResults bool
}

// Target describes a user-selected region of code to operate on.

// It can represent either an entire file or a specific function/line range.
// It is provided by the caller and used to scope LLM-based operations
// such as refactoring, explanation, or test generation.
type Target struct {
	File      string
	Function  string
	StartLine int
	EndLine   int
}

// FlushMemory processes all pending memory queue items.
//
// It loads persisted interactions, runs extraction, updates the memory store,
// and removes successfully processed entries. It returns the number of items
// successfully processed. Processing is best-effort and may take time depending
// on LLM latency.
func (c *Client) FlushMemory(ctx context.Context) (int, error) {
	return c.memory.FlushMemory(ctx)
}

// Ask sends a raw prompt directly to the LLM and streams the response.
//
// This bypasses the index and does not perform any retrieval.
// Supported TaskOptions:
//   - WithWebSearch
//   - WithResultHandler
func (c *Client) Ask(ctx context.Context, prompt string, opts ...TaskOption) error {
	if prompt == "" {
		return ErrEmptyPrompt
	}

	_, err := c.engine.Run(ctx, engine.Request{
		Kind:       engine.TaskAsk,
		UserPrompt: prompt,
		Template:   prompts.TemplateAsk,
		Config:     parseTaskOptions(opts...),
	},
	)
	return err
}

// Search performs a semantic search against the index and writes
// the matching results to the configured writer.
//
// It does not invoke the LLM.
func (c *Client) Search(ctx context.Context, prompt string, opts ...TaskOption) (TaskResult, error) {
	if prompt == "" {
		return TaskResult{}, ErrEmptyPrompt
	}

	r, err := c.engine.Run(ctx, engine.Request{
		Kind:       engine.TaskSearch,
		UserPrompt: prompt,
		Config:     parseTaskOptions(opts...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Query executes a retrieval-augmented query against the index
// and streams the generated answer.
//
// It enhances the query using repository summary when needed and
// may retry the search if initial results are insufficient.
//
// Behavior:
//   - performs semantic search
//   - optionally enriches the query with repository summary
//   - retries if results are weak or empty
//   - injects results into a prompt template
//   - streams the final answer via the LLM
//
// If no relevant results are found, TaskResult.Status.NoResults is set.
func (c *Client) Query(ctx context.Context, prompt string, opts ...TaskOption) (TaskResult, error) {
	if prompt == "" {
		return TaskResult{}, ErrEmptyPrompt
	}

	r, err := c.engine.Run(ctx, engine.Request{
		Kind:       engine.TaskQuery,
		UserPrompt: prompt,
		Template:   prompts.TemplateQuery,
		Config:     parseTaskOptions(opts...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Summarize generates a summary of a Target file.
// It disables retrieval by default.
func (c *Client) Summarize(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	r, err := c.engine.Run(ctx, engine.Request{
		Kind:     engine.TaskTarget,
		Target:   engine.Target(target),
		Template: prompts.TemplateSummarize,
		Config:   parseTaskOptions(withDefaultRetrieval(RetrievalNone, opts)...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Explain analyzes a target and generates an explanation using
// relevant context retrieved from the index.

// It attempts to identify related code (dependencies, symbols)
// and excludes the target file itself from the retrieved context
// to avoid redundant information.
func (c *Client) Explain(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	r, err := c.engine.Run(ctx, engine.Request{
		Kind:     engine.TaskTarget,
		Target:   engine.Target(target),
		Template: prompts.TemplateExplain,
		Config:   parseTaskOptions(opts...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// GenerateTests generates tests for a given target.
//
// It retrieves relevant context from the index and uses a language-
// specific template when supported.
func (c *Client) GenerateTests(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	template := prompts.TemplateTestGeneric
	if isSupportedLanguage(target.File) {
		template = prompts.TemplateTestGo
	}

	r, err := c.engine.Run(ctx, engine.Request{
		Kind:     engine.TaskTarget,
		Target:   engine.Target(target),
		Template: template,
		Config:   parseTaskOptions(withDefaultRetrieval(RetrievalNone, opts)...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Edit modifies code based on a user-provided instruction.
//
// It loads the target source file, optionally narrows the context to a specific
// function, and combines it with the user’s prompt to guide an LLM-based edit.
//
// The operation may include additional contextual retrieval (if enabled via opts)
// to improve multi-file or dependency-aware changes.
//
// This is a general-purpose code transformation primitive used for fixes,
// refactors, and targeted rewrites.
func (c *Client) Edit(ctx context.Context, target Target, prompt string, opts ...TaskOption) (TaskResult, error) {
	r, err := c.engine.Run(ctx, engine.Request{
		Kind:       engine.TaskTarget,
		UserPrompt: prompt,
		Target:     engine.Target(target),
		Template:   prompts.TemplateEdit,
		Config:     parseTaskOptions(opts...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Fix attempts to correct bugs or incorrect behavior in the given target.
// It applies minimal changes required to restore correctness without refactoring.
func (c *Client) Fix(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	r, err := c.engine.Run(ctx, engine.Request{
		Kind:     engine.TaskTarget,
		Target:   engine.Target(target),
		Template: prompts.TemplateFix,
		Config:   parseTaskOptions(opts...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Refactor improves code structure and readability without changing behavior.
// It focuses on maintainability, clarity, and idiomatic style.
func (c *Client) Refactor(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	r, err := c.engine.Run(ctx, engine.Request{
		Kind:     engine.TaskTarget,
		Target:   engine.Target(target),
		Template: prompts.TemplateRefactor,
		Config:   parseTaskOptions(opts...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// Comment adds or improves code comments to explain intent and non-obvious logic.
// It does not modify code behavior.
func (c *Client) Comment(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	r, err := c.engine.Run(ctx, engine.Request{
		Kind:     engine.TaskTarget,
		Target:   engine.Target(target),
		Template: prompts.TemplateComment,
		Config:   parseTaskOptions(withDefaultRetrieval(RetrievalNone, opts)...),
	})
	return TaskResult{Status: TaskStatus{NoResults: r.NoResults}}, err
}

// isSupportedLanguage reports whether test generation is supported
// for the given file type.
func isSupportedLanguage(path string) bool {
	switch filepath.Ext(path) {
	case ".go":
		return true
	}
	return false
}
