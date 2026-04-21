package ai

import (
	"context"
	"coolaid/internal/core/engine"
	"coolaid/internal/indexer"
	"coolaid/internal/prompts"
	"errors"
	"log/slog"
	"path/filepath"
	"time"
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

// IndexProgress represents the current progress of an indexing operation.
//
// It is emitted during Client.Index via the optional progress callback.
// Done indicates the number of files processed so far, out of Total.
// File is the current file being indexed, and Size is its size in bytes.
type IndexProgress struct {
	Done  int64
	Total int64
	File  string
	Size  int64
}

// IndexResult summarizes the outcome of a completed indexing operation.
//
// It is passed to the onComplete callback of Client.Index and provides
// high-level information about the indexing process, such as the number
// of chunks stored, the database location, and the total elapsed time.
type IndexResult struct {
	Chunks        int
	StoreLocation string
	Elapsed       time.Duration
}

// Index builds a fresh index for the configured project.
//
// It clears any existing index, scans the project, generates embeddings,
// and persists the results to the store.
//
// Progress updates are reported via the onProgress callback (if provided).
// When indexing completes successfully, onComplete is invoked with a summary
// of the operation (if provided).
//
// This method does not control how progress or results are rendered;
// that responsibility is left to the caller.
func (c *Client) Index(ctx context.Context, onProgress func(IndexProgress), onComplete func(IndexResult)) error {
	if err := c.store.ResetIndex(); err != nil {
		return err
	}

	c.cfg.Logger.Info("Indexing project", slog.String("root", c.store.ProjectRoot))
	start := time.Now()

	opts := indexer.IndexOptions{
		IgnorePatterns: c.cfg.IgnorePatterns,
		Extensions:     c.cfg.extensions,
		MaxWorkers:     c.cfg.IndexMaxWorkers,
	}
	if err := indexer.Build(ctx, c.llm, c.store, c.cfg.Logger, opts, func(p indexer.Progress) {
		if onProgress != nil {
			onProgress(IndexProgress{
				Done:  p.Done,
				Total: p.Total,
				File:  p.File,
				Size:  p.Size,
			})
		}
	}); err != nil {
		return err
	}

	if err := c.store.Save(); err != nil {
		return err
	}

	elapsed := time.Since(start)
	if onComplete != nil {
		onComplete(IndexResult{
			Chunks:        len(c.store.Items),
			StoreLocation: c.store.DBPath,
			Elapsed:       elapsed,
		})
	}

	c.cfg.Logger.Info("Indexing completed",
		slog.Int("chunks", len(c.store.Items)),
		slog.String("store_location", c.store.DBPath),
		slog.Duration("elapsed_time", elapsed),
	)
	return nil
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
