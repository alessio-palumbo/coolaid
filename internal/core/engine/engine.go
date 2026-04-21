package engine

import (
	"cmp"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"slices"
	"strings"

	"coolaid/internal/llm"
	"coolaid/internal/prompts"
	"coolaid/internal/query"
	"coolaid/internal/retrieval"
	"coolaid/internal/store"
	"coolaid/internal/web"
)

// minAcceptableScore defines the minimum similarity score required
// for a search result to be considered relevant.
//
// Results below this threshold may trigger a retry with additional
// context (e.g. repository summary).
const minAcceptableScore = 0.5

// LLM defines the language model operations used by task execution,
// supporting both buffered and streaming generation, as well as chat as embedding.
type LLM interface {
	Generate(ctx context.Context, prompt string) (string, error)
	GenerateStream(ctx context.Context, prompt string, writer io.Writer) error
	ChatStream(ctx context.Context, messages []llm.Message, writer io.Writer) (string, error)
	Embed(ctx context.Context, text string) ([]float64, error)
}

// SearchStore defines the storage operations required by Engine
// for retrieval, symbol lookup, and repository summary access.
type SearchStore interface {
	GetMemory() store.Memory
	GetSummary() (string, error)
	Search([]float64, int, bool) ([]retrieval.Chunk, error)
	FindBySymbol(string, string, int) ([]retrieval.Chunk, error)
}

// Memory defines the interface for asynchronous project memory management.
//
// It accepts interaction inputs for background processing and supports
// graceful shutdown of any internal workers.
type Memory interface {
	Capture(w io.Writer, userPrompt string, fn func(w io.Writer) error) error
	FlushMemory(ctx context.Context) (int, error)
}

// Request represents a single execution request handled by Engine.
//
// It encapsulates the task kind, prompt or target inputs, prompt template,
// and task-specific options needed to run a pipeline.
type Request struct {
	Kind       TaskKind
	UserPrompt string
	Target     Target
	Template   prompts.Template
	Config     *TaskConfig
}

type Result struct {
	NoResults bool
}

// TaskKind identifies which execution pipeline Engine should run.
//
// It selects behavior such as direct generation, retrieval-augmented query,
// semantic search, or target-based code analysis.
type TaskKind string

const (
	TaskAsk    TaskKind = "ask"
	TaskQuery  TaskKind = "query"
	TaskSearch TaskKind = "search"
	TaskTarget TaskKind = "target"
)

// Engine coordinates task execution by composing retrieval, memory,
// prompt construction, and LLM generation into reusable pipelines.
type Engine struct {
	llm    LLM
	store  SearchStore
	memory Memory
	writer io.Writer
}

// NewEngine constructs an Engine with the dependencies required
// to execute prompts, retrieval, and memory-aware task pipelines.
func NewEngine(llm LLM, store SearchStore, mem Memory, writer io.Writer) *Engine {
	return &Engine{
		llm:    llm,
		store:  store,
		memory: mem,
		writer: writer,
	}
}

// Run dispatches a Request to the appropriate execution pipeline
// based on its TaskKind.
func (e *Engine) Run(ctx context.Context, req Request) (Result, error) {
	t := newTask(req.UserPrompt, req.Target, req.Template, req.Config)

	switch req.Kind {
	case TaskAsk:
		return e.runAsk(ctx, t)
	case TaskQuery:
		return e.runQuery(ctx, t)
	case TaskSearch:
		return e.runSearch(ctx, t)
	default:
		return e.runTargetTask(ctx, t)
	}
}

// ChatRequest represents a single turn in a stateful chat session.
//
// It contains the user message, prior conversation history, and
// task configuration used to control retrieval, prompt behavior,
// and LLM output formatting.
//
// History is treated as immutable input and must not be mutated
// by the engine.
type ChatRequest struct {
	Msg     string
	History []llm.Message
	Config  *TaskConfig
}

// RunChat executes a single turn of a conversational interaction.
//
// It performs retrieval over the codebase, builds a chat prompt with
// optional memory and system overrides, and streams the response via
// the underlying LLM chat interface.
//
// The returned string is the final assistant message for this turn.
func (e *Engine) RunChat(ctx context.Context, req ChatRequest) (string, error) {
	chunks, err := e.semanticSearch(ctx, req.Msg, req.Config.Retrieval.K, req.Config.Retrieval.UseMMR)
	if err != nil {
		return "", err
	}

	renderedPrompt, err := prompts.Render(&prompts.Config{
		Template:       prompts.TemplateChat,
		SystemOverride: req.Config.Prompt.SystemOverride,
		Structured:     req.Config.Prompt.StructuredOutput,
		Memory:         e.store.GetMemory(),
	}, req.Msg, chunks...)
	if err != nil {
		return "", err
	}

	var assistantResp string
	err = e.memory.Capture(e.writer, req.Msg, func(w io.Writer) error {
		userMsg := llm.Message{Role: llm.RoleUser, Content: renderedPrompt}
		resp, err := e.llm.ChatStream(ctx, append(req.History, userMsg), w)
		if err != nil {
			return err
		}
		assistantResp = resp
		return nil
	})
	return assistantResp, err
}

// runAsk executes a direct prompt task, optionally augmenting the
// prompt with web retrieval before invoking the LLM.
func (e *Engine) runAsk(ctx context.Context, t task) (Result, error) {
	var chunks []retrieval.Chunk
	if l := t.config.Web.SearchLimit; l > 0 {
		results, err := web.NewPipeline(l).Run(ctx, t.UserPrompt)
		if err != nil {
			return Result{}, err
		}
		chunks = results
	}

	renderedPrompt, err := t.buildPrompt(e.store.GetMemory(), chunks...)
	if err != nil {
		return Result{}, err
	}

	err = e.memory.Capture(e.writer, t.UserPrompt, func(w io.Writer) error {
		return t.execute(ctx, e.llm, w, renderedPrompt)
	},
	)

	return Result{}, err
}

// runQuery executes a retrieval-augmented query against indexed code,
// builds a prompt from retrieved context, and invokes the LLM.
func (e *Engine) runQuery(ctx context.Context, t task) (Result, error) {
	searchPrompt := t.UserPrompt
	if !query.IsSearchable(searchPrompt) {
		summary, err := e.store.GetSummary()
		if err != nil {
			return Result{}, err
		}
		searchPrompt = enrichWithSummary(searchPrompt, summary)
		t.Summary = summary
	}

	chunks, err := e.doSearch(ctx, searchPrompt, t.config.Retrieval, false)
	if err != nil {
		return Result{}, err
	}

	if shouldRetry(chunks) && t.Summary == "" {
		// Safe to ignore error as doSearch already loads the DB.
		t.Summary, _ = e.store.GetSummary()
		searchPrompt = enrichWithSummary(searchPrompt, t.Summary)

		chunks, err = e.semanticSearch(ctx, searchPrompt, t.config.Retrieval.K, t.config.Retrieval.UseMMR)
		if err != nil {
			return Result{}, err
		}
	}
	if len(chunks) == 0 {
		return Result{NoResults: true}, nil
	}

	renderedPrompt, err := t.buildPrompt(e.store.GetMemory(), chunks...)
	if err != nil {
		return Result{}, err
	}

	return Result{}, e.memory.Capture(e.writer, t.UserPrompt, func(w io.Writer) error {
		return t.execute(ctx, e.llm, w, renderedPrompt)
	})
}

// runSearch executes semantic retrieval only and returns matching
// chunks without invoking the LLM.
func (e *Engine) runSearch(ctx context.Context, t task) (Result, error) {
	results, err := e.doSearch(ctx, t.UserPrompt, t.config.Retrieval, false)
	if err != nil {
		return Result{}, err
	}
	if len(results) == 0 {
		return Result{NoResults: true}, nil
	}

	fmt.Fprint(e.writer, retrieval.JoinChunks(results...))
	return Result{}, nil
}

// runTargetTask executes a target-based task by loading the target,
// retrieving supporting context, building a prompt, and invoking the LLM.
func (e *Engine) runTargetTask(ctx context.Context, t task) (Result, error) {
	chunks, data, err := e.retrieveFromTarget(ctx, t.Target, t.config)
	if err != nil {
		return Result{}, err
	}

	t.TargetBody = extractTarget(data, t.Target)
	prompt, err := t.buildPrompt(e.store.GetMemory(), chunks...)
	if err != nil {
		return Result{}, err
	}

	return Result{}, t.execute(ctx, e.llm, e.writer, prompt)
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
func (e *Engine) retrieveFromTarget(ctx context.Context, target Target, taskCfg *TaskConfig) ([]retrieval.Chunk, []byte, error) {
	if err := target.validate(); err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(target.File)
	if err != nil {
		return nil, nil, err
	}

	if taskCfg.Retrieval.K < 1 {
		return nil, data, nil
	}

	// Find dependencies chunks to pass as dependencies to LLM.
	signals := query.ExtractSignals(target.File, data, false)
	results, err := e.doSearch(ctx, signals, taskCfg.Retrieval, false)
	return results, data, err
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
func (e *Engine) doSearch(ctx context.Context, userPrompt string, opts RetrievalOptions, preferSymbol bool) ([]retrieval.Chunk, error) {
	var symbolResults []retrieval.Chunk
	for _, sym := range query.ExtractIdentifiers(userPrompt) {
		results, err := e.store.FindBySymbol(userPrompt, sym, opts.K)
		if err != nil {
			return nil, err
		}
		symbolResults = append(symbolResults, results...)
	}

	if len(symbolResults) > 0 && (preferSymbol || len(symbolResults) >= opts.K) {
		return symbolResults, nil
	}

	// compute semantic budget
	semanticK := opts.K
	if len(symbolResults) > 0 {
		semanticK -= len(symbolResults)
	}
	semanticResults, err := e.semanticSearch(ctx, userPrompt, semanticK, opts.UseMMR)
	if err != nil {
		return nil, err
	}
	if len(symbolResults) == 0 {
		return semanticResults, nil
	}
	return mergeResults(symbolResults, semanticResults, opts.K), nil
}

// semanticSearch performs a vector similarity search against the index.
//
// It embeds the prompt and retrieves the top-k most relevant chunks.
// If useMMR is true, Max Marginal Relevance is applied to improve diversity.
func (e *Engine) semanticSearch(ctx context.Context, userPrompt string, k int, useMMR bool) ([]retrieval.Chunk, error) {
	queryVec, err := e.llm.Embed(ctx, userPrompt)
	if err != nil {
		return nil, err
	}
	return e.store.Search(queryVec, k, useMMR)
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
