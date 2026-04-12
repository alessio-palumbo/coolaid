package ai

import (
	"cmp"
	"context"
	"coolaid/internal/indexer"
	"coolaid/internal/prompts"
	"coolaid/internal/query"
	"coolaid/internal/retrieval"
	"coolaid/internal/web"
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

var (
	// ErrEmptyPrompt is returned when a task requires a non-empty prompt.
	ErrEmptyPrompt = errors.New("prompt required")

	// ErrTargetFileRequired is returned when a task requires a target file:[fn].
	ErrTargetFileRequired = errors.New("target file required")
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
	if err := indexer.Build(c.llm, c.store, c.cfg.Logger, opts, func(p indexer.Progress) {
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

type AskOptions struct {
	UseWeb      bool
	SearchLimit int
}

// Ask sends a raw prompt directly to the LLM and streams the response.
//
// This bypasses the index and does not perform any retrieval.
func (c *Client) Ask(ctx context.Context, prompt string, opts AskOptions) error {
	if prompt == "" {
		return ErrEmptyPrompt
	}

	if opts.UseWeb && opts.SearchLimit > 0 {
		chunks, err := web.NewPipeline(opts.SearchLimit).Run(ctx, prompt)
		if err != nil {
			return err
		}

		prompt, err = prompts.Render(
			&prompts.Config{Template: prompts.TemplateQuery},
			prompt, chunks...,
		)
		if err != nil {
			return err
		}
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

	return c.llm.GenerateStream(prompt, c.writer)
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
	results, err := c.doSearch(ctx, prompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR, false)
	if err != nil {
		return TaskResult{}, err
	}
	if len(results) == 0 {
		return TaskResult{Status: TaskStatus{NoResults: true}}, nil
	}

	fmt.Fprint(c.writer, retrieval.JoinChunks(results...))
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

	results, err := c.doSearch(ctx, searchPrompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR, false)
	if err != nil {
		return TaskResult{}, err
	}

	if shouldRetry(results) && !usedSummary {
		searchPrompt = enrichWithSummary(searchPrompt, c.store.Summary)
		usedSummary = true

		results, err = c.semanticSearch(ctx, searchPrompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
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

	return TaskResult{}, c.llm.GenerateStream(renderedPrompt, c.writer)
}

// Explain analyzes a target and generates an explanation using
// relevant context retrieved from the index.

// It attempts to identify related code (dependencies, symbols)
// and excludes the target file itself from the retrieved context
// to avoid redundant information.
func (c *Client) Explain(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	return c.runTargetTask(ctx, target, "", prompts.TemplateExplain, opts)
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
	return c.runTargetTask(ctx, target, "", template, opts)
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
	return c.runTargetTask(ctx, target, prompt, prompts.TemplateEdit, opts)
}

// Fix attempts to correct bugs or incorrect behavior in the given target.
// It applies minimal changes required to restore correctness without refactoring.
func (c *Client) Fix(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	return c.runTargetTask(ctx, target, "", prompts.TemplateFix, opts)
}

// Refactor improves code structure and readability without changing behavior.
// It focuses on maintainability, clarity, and idiomatic style.
func (c *Client) Refactor(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	return c.runTargetTask(ctx, target, "", prompts.TemplateRefactor, opts)
}

// Comment adds or improves code comments to explain intent and non-obvious logic.
// It does not modify code behavior.
func (c *Client) Comment(ctx context.Context, target Target, opts ...TaskOption) (TaskResult, error) {
	return c.runTargetTask(ctx, target, "", prompts.TemplateComment, opts)
}

// runTargetTask is a shared helper for target-based LLM tasks.
//
// It loads the target file, optionally retrieves supporting context (based on
// retrieval settings), and renders a prompt using the provided template.
//
// The target body (file or function) is always included, while retrieved chunks
// are passed as optional context to improve reasoning when enabled.
//
// This function centralizes prompt construction and execution, ensuring
// consistent behavior across all target-based commands.
func (c *Client) runTargetTask(ctx context.Context, target Target, prompt string, template prompts.PromptTemplate, opts []TaskOption) (TaskResult, error) {
	taskCfg := parseTaskOptions(opts...)

	results, data, err := c.retrieveFromTarget(ctx, target, taskCfg)
	if err != nil {
		return TaskResult{}, err
	}

	// Exclude any chunks matching the file to avoid wasting tokens unless function is defined.
	if target.Function == "" {
		for i := 0; i < len(results); i++ {
			if strings.Contains(results[i].Source, target.File) {
				results = slices.Delete(results, i, i+1)
				i--
			}
		}
	}

	pConfig := &prompts.Config{
		Template:       template,
		SystemOverride: taskCfg.prompt.systemOverride,
		Structured:     taskCfg.prompt.structuredOutput,
	}
	renderedPrompt, err := prompts.Render(
		pConfig.WithTarget(target.File, target.Function, extractTarget(data, target)),
		prompt, results...,
	)
	fmt.Println(renderedPrompt)
	if err != nil {
		return TaskResult{}, err
	}

	return TaskResult{}, c.llm.GenerateStream(renderedPrompt, c.writer)
}

// doSearch retrieves relevant chunks for a given prompt using a symbol-first strategy.
//
// It first attempts to extract a code-like identifier (e.g. "TestCommand")
// and performs a fast in-memory symbol lookup. Symbol matches are high-precision
// and are preferred when available.
//
// If symbol matches are found:
//   - When preferSymbol=true, they are returned immediately.
//   - When the number of symbol matches satisfies k, they are returned immediately.
//   - Otherwise, they are combined with semantic search results to fill the remaining slots.
//
// If no symbol is found (or no symbol matches exist), it falls back to semantic
// (vector) search.
//
// This approach balances deterministic identifier-based retrieval with semantic
// similarity, while limiting noise and preserving result count expectations.
func (c *Client) doSearch(ctx context.Context, prompt string, k int, useMMR bool, preferSymbol bool) ([]retrieval.Chunk, error) {
	var symbolResults []retrieval.Chunk
	for _, sym := range query.ExtractIdentifiers(prompt) {
		results, err := c.store.FindBySymbol(prompt, sym, k)
		if err != nil {
			return nil, err
		}
		symbolResults = append(symbolResults, results...)
	}

	if len(symbolResults) > 0 && (preferSymbol || len(symbolResults) >= k) {
		return symbolResults, nil
	}

	// compute semantic budget
	semanticK := k
	if len(symbolResults) > 0 {
		semanticK = k - len(symbolResults)
	}
	semanticResults, err := c.semanticSearch(ctx, prompt, semanticK, useMMR)
	if err != nil {
		return nil, err
	}
	if len(symbolResults) == 0 {
		return semanticResults, nil
	}
	return mergeResults(symbolResults, semanticResults, k), nil
}

// semanticSearch performs a vector similarity search against the index.
//
// It embeds the prompt and retrieves the top-k most relevant chunks.
// If useMMR is true, Max Marginal Relevance is applied to improve diversity.
func (c *Client) semanticSearch(ctx context.Context, prompt string, k int, useMMR bool) ([]retrieval.Chunk, error) {
	queryVec, err := c.llm.Embed(prompt)
	if err != nil {
		return nil, err
	}
	return c.store.Search(queryVec, k, useMMR)
}

// retrieveFromTarget builds retrieval context for a given Target.
//
// It validates the target, reads the file content, and optionally performs
// semantic retrieval over the indexed codebase using extracted signals
// (e.g. identifiers, structure, comments).
//
// The returned chunks represent optional supporting context (related functions,
// types, or usages) that may improve LLM reasoning when combined with the target.
//
// The raw file content is always returned so callers can extract the specific
// function or file segment for the prompt.
//
// Retrieval is disabled when k < 1 (e.g. RetrievalNone), in which case only
// the file content is returned.
//
// Note: This is a best-effort enrichment step. Failure or empty results should
// not prevent LLM execution.
func (c *Client) retrieveFromTarget(ctx context.Context, target Target, taskCfg *taskConfig) ([]retrieval.Chunk, []byte, error) {
	if err := target.validate(); err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(target.File)
	if err != nil {
		return nil, nil, err
	}

	if taskCfg.retrieval.k < 1 {
		return nil, data, nil
	}

	// Find dependencies chunks to pass as dependencies to LLM.
	signals := query.ExtractSignals(target.File, data, false)
	results, err := c.doSearch(ctx, signals, taskCfg.retrieval.k, taskCfg.retrieval.useMMR, false)
	return results, data, err
}

// enrichWithSummary appends repository summary context to the prompt.
func enrichWithSummary(prompt, summary string) string {
	return prompt + "\n\n" + summary
}

// shouldRetry determines whether a search should be retried with
// additional context (e.g. summary) based on chunk quality.
func shouldRetry(results []retrieval.Chunk) bool {
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

// extractTarget returns a slice of the source based on Target.
// Priority: range > function > full file.
func extractTarget(src []byte, t Target) string {
	// Range-based extraction (line range)
	if t.StartLine > 0 && t.EndLine > 0 {
		lines := strings.Split(string(src), "\n")

		// clamp bounds
		start := max(t.StartLine-1, 0)
		end := min(t.EndLine, len(lines))

		return strings.Join(lines[start:end], "\n")
	}

	// Function-based extraction
	if t.Function != "" {
		// TODO support extraction for other languages through treesitter.
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
		if err == nil {
			for _, decl := range file.Decls {
				f, ok := decl.(*ast.FuncDecl)
				if !ok || f.Name.Name != t.Function {
					continue
				}

				start := fset.Position(f.Pos()).Offset
				if f.Doc != nil {
					start = fset.Position(f.Doc.Pos()).Offset
				}
				end := fset.Position(f.End()).Offset

				return string(src[start:end])
			}
		}
	}

	// Fallback: full file
	return string(src)
}

// mergeResults combines symbol-based and semantic search results into a single ranked list.
//
// Chunks are appended and then sorted by score in descending order. The final slice
// is truncated to at most k elements.
//
// It assumes both input slices use the same scoring scale (e.g. cosine similarity in [0,1]).
// Symbol chunks are typically higher precision and expected to rank above semantic chunks,
// but no explicit boosting is applied here.
//
// This function does not deduplicate chunks. Callers should ensure inputs are distinct
// if necessary.
func mergeResults(sym, vec []retrieval.Chunk, k int) []retrieval.Chunk {
	results := append(sym, vec...)
	slices.SortFunc(results, func(a, b retrieval.Chunk) int {
		return cmp.Compare(b.Score, a.Score)
	})

	if len(results) > k {
		results = results[:k]
	}
	return results
}
