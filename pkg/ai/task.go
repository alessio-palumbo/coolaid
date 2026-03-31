package ai

import (
	"ai-cli/internal/indexer"
	"ai-cli/internal/prompts"
	"ai-cli/internal/query"
	"ai-cli/internal/vector"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// minAcceptableScore defines the minimum similarity score required
// for a search result to be considered relevant.
//
// Results below this threshold may trigger a retry with additional
// context (e.g. repository summary).
const minAcceptableScore = 0.5

// ErrEmptyPrompt is returned when a task requires a non-empty prompt.
var ErrEmptyPrompt = errors.New("prompt required")

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
	if err := c.store.Clear(); err != nil {
		return err
	}

	c.logger.Info("Indexing project", slog.String("root", c.store.ProjectRoot))
	start := time.Now()

	onProgressFunc := func(p indexer.Progress) {
		if onProgress != nil {
			onProgress(IndexProgress{
				Done:  p.Done,
				Total: p.Total,
				File:  p.File,
				Size:  p.Size,
			})
		}
	}
	if err := indexer.Build(c.cfg, c.llm, c.store, onProgressFunc); err != nil {
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

	c.logger.Info("Indexing completed",
		slog.Int("chunks", len(c.store.Items)),
		slog.String("store_location", c.store.DBPath),
		slog.Duration("elapsed_time", elapsed),
	)
	return nil
}

// Ask sends a raw prompt directly to the LLM and streams the response.
//
// This bypasses the index and does not perform any retrieval.
func (c *Client) Ask(ctx context.Context, prompt string) error {
	if prompt == "" {
		return ErrEmptyPrompt
	}
	return c.llm.GenerateStream(prompt, c.writer)
}

// Summarize generates a summary of the given file.
//
// The file content is passed directly to the LLM without retrieval.
func (c *Client) Summarize(ctx context.Context, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	prompt, err := prompts.Render(&prompts.Config{Template: prompts.TemplateSummarize}, string(data))
	if err != nil {
		return err
	}

	return c.llm.GenerateStream(prompt, os.Stdout)
}

// Explain analyzes a file and generates an explanation using
// relevant context retrieved from the index.
//
// It attempts to identify related code (dependencies, symbols)
// and excludes the target file itself from the retrieved context
// to avoid redundant information.
func (c *Client) Explain(ctx context.Context, file string, opts ...TaskOption) (TaskResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return TaskResult{}, err
	}

	// Find dependencies chunks to pass as dependencies to LLM.
	signals := query.ExtractSignals(file, data)
	taskCfg := parseTaskOptions(opts...)
	results, err := c.SemanticSearch(ctx, signals, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
	if err != nil {
		return TaskResult{}, err
	}
	if len(results) == 0 {
		return TaskResult{Status: TaskStatus{NoResults: true}}, nil
	}

	// Exclude any chunks matching the file to avoid wasting tokens.
	for i, r := range results {
		if strings.Contains(r.FilePath, file) {
			results = slices.Delete(results, i, i+1)
		}
	}

	content := "file: " + file + "\n\n" + string(data)
	prompt, err := prompts.Render(&prompts.Config{Template: prompts.TemplateExplain}, content, results...)
	if err != nil {
		return TaskResult{}, err
	}

	return TaskResult{}, c.llm.GenerateStream(prompt, os.Stdout)
}

// Search performs a semantic search against the index and writes
// the matching results to the configured writer.
//
// It does not invoke the LLM.
func (c *Client) Search(ctx context.Context, prompt string, opts ...TaskOption) (TaskResult, error) {
	if prompt == "" {
		return TaskResult{}, ErrEmptyPrompt
	}

	taskCfg := parseTaskOptions(opts...)
	results, err := c.SemanticSearch(ctx, prompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
	if err != nil {
		return TaskResult{}, err
	}
	if len(results) == 0 {
		return TaskResult{Status: TaskStatus{NoResults: true}}, nil
	}

	fmt.Fprint(c.writer, vector.JoinResults(results...))
	return TaskResult{}, nil
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

	taskCfg := parseTaskOptions(opts...)
	searchPrompt := prompt

	var usedSummary bool
	if !query.IsSearchable(searchPrompt) {
		// Make sure the Summary is present before appending.
		c.store.EnsureLoaded()
		searchPrompt = enrichWithSummary(searchPrompt, c.store.Summary)
		usedSummary = true
	}

	results, err := c.SemanticSearch(ctx, searchPrompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
	if err != nil {
		return TaskResult{}, err
	}

	if shouldRetry(results) && !usedSummary {
		searchPrompt = enrichWithSummary(searchPrompt, c.store.Summary)
		usedSummary = true

		results, err = c.SemanticSearch(ctx, searchPrompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
		if err != nil {
			return TaskResult{}, err
		}
	}
	if len(results) == 0 {
		return TaskResult{Status: TaskStatus{NoResults: true}}, nil
	}

	pConfig := &prompts.Config{
		Template:       prompts.TemplateQuery,
		SystemOverride: taskCfg.prompt.systemOverride,
		Structured:     taskCfg.prompt.structuredOutput,
	}
	if usedSummary {
		pConfig.Summary = c.store.Summary
	}
	renderedPrompt, err := prompts.Render(pConfig, prompt, results...)
	if err != nil {
		return TaskResult{}, err
	}

	return TaskResult{}, c.llm.GenerateStream(renderedPrompt, os.Stdout)
}

// GenerateTests generates tests for a given target.
//
// The target can be:
//   - a file path (e.g. "file.go")
//   - a file and function (e.g. "file.go:FuncName")
//
// It retrieves relevant context from the index and uses a language-
// specific template when supported.
func (c *Client) GenerateTests(ctx context.Context, target string, opts ...TaskOption) (TaskResult, error) {
	if target == "" {
		return TaskResult{}, fmt.Errorf("target required")
	}

	file, fn := parseTarget(target)
	data, err := os.ReadFile(file)
	if err != nil {
		return TaskResult{}, err
	}

	signals := query.ExtractSignals(file, data)
	taskCfg := parseTaskOptions(opts...)
	results, err := c.SemanticSearch(ctx, signals, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
	if err != nil {
		return TaskResult{}, err
	}
	if len(results) == 0 {
		return TaskResult{Status: TaskStatus{NoResults: true}}, nil
	}

	pConfig := &prompts.Config{
		SystemOverride: taskCfg.prompt.systemOverride,
		Structured:     taskCfg.prompt.structuredOutput,
	}
	if isSupportedLanguage(file) {
		pConfig.Template = prompts.TemplateTestGo
	} else {
		pConfig.Template = prompts.TemplateTestGeneric
	}

	prompt, err := prompts.Render(pConfig, extractTarget(data, fn), results...)
	if err != nil {
		return TaskResult{}, err
	}

	return TaskResult{}, c.llm.GenerateStream(prompt, c.writer)
}

// SemanticSearch performs a vector similarity search against the index.
//
// It embeds the prompt and retrieves the top-k most relevant results.
// If useMMR is true, Max Marginal Relevance is applied to improve diversity.
func (c *Client) SemanticSearch(ctx context.Context, prompt string, k int, useMMR bool) ([]vector.Result, error) {
	queryVec, err := c.llm.Embed(prompt)
	if err != nil {
		return nil, err
	}
	return c.store.Search(queryVec, k, useMMR)
}

// enrichWithSummary appends repository summary context to the prompt.
func enrichWithSummary(prompt, summary string) string {
	return prompt + "\n\n" + summary
}

// shouldRetry determines whether a search should be retried with
// additional context (e.g. summary) based on result quality.
func shouldRetry(results []vector.Result) bool {
	return len(results) == 0 || results[0].Score < minAcceptableScore
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

// parseTarget splits input like "file.go:FuncName" into file path and function name.
// If no function is specified, fn will be empty.
func parseTarget(arg string) (file string, fn string) {
	parts := strings.SplitN(arg, ":", 2)
	file = parts[0]

	if len(parts) == 2 {
		fn = strings.TrimSpace(parts[1])
	}
	return file, fn
}

// extractTarget returns either the full file content or a specific function body.
//
// If fn is empty, the full content is returned.
// If fn is not found, it falls back to the full content.
func extractTarget(src []byte, fn string) string {
	if fn == "" {
		return string(src)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return string(src) // fallback
	}

	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if f.Name.Name != fn {
			continue
		}

		start := fset.Position(f.Pos()).Offset
		end := fset.Position(f.End()).Offset
		return string(src[start:end])
	}

	// fallback if function not found
	return string(src)
}
