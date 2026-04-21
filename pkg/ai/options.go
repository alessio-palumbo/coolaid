package ai

import (
	"coolaid/internal/core/engine"
)

// RetrievalMode defines predefined strategies for semantic retrieval.
//
// It controls how many results are fetched and whether diversification
// techniques like MMR (Max Marginal Relevance) are applied.
type RetrievalMode string

const (
	// RetrievalNone disables retrieval entirely.
	// The model operates only on the provided prompt and explicit context (e.g. Target.Body).
	RetrievalNone RetrievalMode = "none"

	// RetrievalFast prioritizes speed with fewer results and no diversification.
	RetrievalFast RetrievalMode = "fast"

	// RetrievalBalanced provides a balance between speed and relevance.
	RetrievalBalanced RetrievalMode = "balanced"

	// RetrievalDeep prioritizes recall and diversity, retrieving more results
	// and enabling MMR for better coverage.
	RetrievalDeep RetrievalMode = "deep"
)

// ResultHandler processes the final generated output after task execution,
// allowing optional post-processing such as file writes or memory updates.
//
// It is defined by the engine and exposed here for API compatibility.
type ResultHandler = engine.ResultHandler

// TaskOption configures the behavior of a task (e.g. Query, Chat, Explain).
//
// It follows the functional options pattern and allows callers to customize
// retrieval and prompt behavior.
type TaskOption func(*engine.TaskConfig)

// WithTopK overrides the number of results retrieved during semantic search.
func WithTopK(k int) TaskOption {
	return func(c *engine.TaskConfig) {
		c.Retrieval.K = k
	}
}

// WithMMR enables or disables Max Marginal Relevance for retrieval.
func WithMMR(enabled bool) TaskOption {
	return func(c *engine.TaskConfig) {
		c.Retrieval.UseMMR = enabled
	}
}

// WithRetrievalMode sets a predefined retrieval strategy.
//
// This overrides both k and MMR settings based on the selected mode.
func WithRetrievalMode(mode RetrievalMode) TaskOption {
	return func(c *engine.TaskConfig) {
		c.Retrieval = defaultRetrievalOptions(mode)
	}
}

// WithNoRetrieval sets retrieval to none.
//
// This overrides both k and MMR settings based on the selected mode.
func WithNoRetrieval() TaskOption {
	return func(c *engine.TaskConfig) {
		c.Retrieval = defaultRetrievalOptions(RetrievalNone)
	}
}

// WithSystemPrompt overrides the default system prompt used for generation.
func WithSystemPrompt(s string) TaskOption {
	return func(c *engine.TaskConfig) {
		c.Prompt.SystemOverride = s
	}
}

// WithStructuredOutput enables structured output generation.
//
// The exact format depends on the prompt template and LLM capabilities.
func WithStructuredOutput() TaskOption {
	return func(c *engine.TaskConfig) {
		c.Prompt.StructuredOutput = true
	}
}

// WithResultHandler registers a post-processing handler to run after
// buffered task execution completes.
func WithResultHandler(h ResultHandler) TaskOption {
	return func(c *engine.TaskConfig) {
		c.Handlers = append(c.Handlers, h)
	}
}

// WithWebSearch enables web search augmentation and sets the maximum
// number of search results to include in the prompt.
func WithWebSearch(searchLimit int) TaskOption {
	return func(c *engine.TaskConfig) {
		c.Web.SearchLimit = searchLimit
	}
}

// withDefaultRetrieval prepends a default retrieval mode so it applies
// unless overridden by a later caller-provided retrieval option.
func withDefaultRetrieval(mode RetrievalMode, opts []TaskOption) []TaskOption {
	return append(
		[]TaskOption{WithRetrievalMode(mode)},
		opts...,
	)
}

// defaultRetrievalOptions returns default retrieval settings for a given mode.
func defaultRetrievalOptions(mode RetrievalMode) engine.RetrievalOptions {
	switch mode {
	case RetrievalNone:
		return engine.RetrievalOptions{K: 0, UseMMR: false}
	case RetrievalDeep:
		return engine.RetrievalOptions{K: 12, UseMMR: true}
	case RetrievalBalanced:
		return engine.RetrievalOptions{K: 8, UseMMR: false}
	default:
		return engine.RetrievalOptions{K: 5, UseMMR: false}
	}
}

// parseTaskOptions applies the provided TaskOptions and returns
// a fully initialized engine.TaskConfig.
//
// By default, RetrievalBalanced mode is used unless overridden.
func parseTaskOptions(opts ...TaskOption) *engine.TaskConfig {
	cfg := engine.TaskConfig{
		Retrieval: defaultRetrievalOptions(RetrievalBalanced),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &cfg
}
