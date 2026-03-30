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
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const minAcceptableScore = 0.5

var ErrEmptyPrompt = errors.New("prompt required")

type TaskResult struct {
	Status TaskStatus
}
type TaskStatus struct {
	NoResults bool
}

func (c *Client) Index(ctx context.Context) error {
	if err := c.store.Clear(); err != nil {
		return err
	}

	fmt.Fprintln(c.writer, "Indexing project at", c.store.ProjectRoot)
	if err := indexer.Build(".", c.store, c.llm, c.cfg); err != nil {
		return err
	}

	if err := c.store.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.writer, "Indexed %d chunks in %s\n", len(c.store.Items), c.store.DBPath)
	return nil
}

func (c *Client) Ask(ctx context.Context, prompt string) error {
	if prompt == "" {
		return ErrEmptyPrompt
	}
	return c.llm.GenerateStream(prompt, c.writer)
}

func (c *Client) Chat(ctx context.Context, prompt string, opts ...TaskOption) error {
	taskCfg := parseTaskOptions(opts...)
	results, err := c.SemanticSearch(ctx, prompt, taskCfg.retrieval.k, taskCfg.retrieval.useMMR)
	if err != nil {
		return err
	}

	renderedPrompt, err := prompts.Render(
		&prompts.Config{Template: prompts.TemplateChat},
		prompt,
		results...,
	)
	if err != nil {
		return err
	}

	return c.llm.ChatStream(renderedPrompt, c.writer)
}

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

// Query executes a semantic query against the index.
// It streams the answer to the configured writer.
//
// If an error is returned, the QueryStatus should be ignored.
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

func (c *Client) SemanticSearch(ctx context.Context, prompt string, k int, useMMR bool) ([]vector.Result, error) {
	queryVec, err := c.llm.Embed(prompt)
	if err != nil {
		return nil, err
	}
	return c.store.Search(queryVec, k, useMMR)
}

func enrichWithSummary(prompt, summary string) string {
	return prompt + "\n\n" + summary
}

func shouldRetry(results []vector.Result) bool {
	return len(results) == 0 || results[0].Score < minAcceptableScore
}

// isSupportedLanguage returns true if the test are supported for the given extension.
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
// If fn is empty, the full content is returned.
// If fn is not found, it falls back to full content.
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
