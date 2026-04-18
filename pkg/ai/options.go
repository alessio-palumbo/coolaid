package ai

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

// TaskOption configures the behavior of a task (e.g. Query, Chat, Explain).
//
// It follows the functional options pattern and allows callers to customize
// retrieval and prompt behavior.
type TaskOption func(*taskConfig)

// WithTopK overrides the number of results retrieved during semantic search.
func WithTopK(k int) TaskOption {
	return func(c *taskConfig) {
		c.retrieval.k = k
	}
}

// WithMMR enables or disables Max Marginal Relevance for retrieval.
func WithMMR(enabled bool) TaskOption {
	return func(c *taskConfig) {
		c.retrieval.useMMR = enabled
	}
}

// WithRetrievalMode sets a predefined retrieval strategy.
//
// This overrides both k and MMR settings based on the selected mode.
func WithRetrievalMode(mode RetrievalMode) TaskOption {
	return func(c *taskConfig) {
		c.retrieval = defaultRetrievalOptions(mode)
	}
}

// WithNoRetrieval sets retrieval to none.
//
// This overrides both k and MMR settings based on the selected mode.
func WithNoRetrieval() TaskOption {
	return func(c *taskConfig) {
		c.retrieval = defaultRetrievalOptions(RetrievalNone)
	}
}

// WithSystemPrompt overrides the default system prompt used for generation.
func WithSystemPrompt(s string) TaskOption {
	return func(c *taskConfig) {
		c.prompt.systemOverride = s
	}
}

// WithStructuredOutput enables structured output generation.
//
// The exact format depends on the prompt template and LLM capabilities.
func WithStructuredOutput() TaskOption {
	return func(c *taskConfig) {
		c.prompt.structuredOutput = true
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

// taskConfig holds internal configuration derived from TaskOptions.
type taskConfig struct {
	retrieval retrievalOptions
	prompt    promptTaskOptions
}

// retrievalOptions controls how semantic search is performed.
type retrievalOptions struct {
	// k is the number of top results to retrieve.
	k int

	// useMMR enables Max Marginal Relevance to improve diversity
	// among retrieved results.
	useMMR bool
}

// promptTaskOptions controls how prompts are constructed for the LLM.
type promptTaskOptions struct {
	// systemOverride replaces the default system prompt used in templates.
	systemOverride string

	// structuredOutput enables structured (machine-readable) responses
	// when supported by the underlying prompt/template.
	structuredOutput bool
}

// defaultRetrievalOptions returns default retrieval settings for a given mode.
func defaultRetrievalOptions(mode RetrievalMode) retrievalOptions {
	switch mode {
	case RetrievalNone:
		return retrievalOptions{k: 0, useMMR: false}
	case RetrievalDeep:
		return retrievalOptions{k: 12, useMMR: true}
	case RetrievalBalanced:
		return retrievalOptions{k: 8, useMMR: false}
	default:
		return retrievalOptions{k: 5, useMMR: false}
	}
}

// parseTaskOptions applies the provided TaskOptions and returns
// a fully initialized taskConfig.
//
// By default, RetrievalBalanced mode is used unless overridden.
func parseTaskOptions(opts ...TaskOption) *taskConfig {
	cfg := taskConfig{
		retrieval: defaultRetrievalOptions(RetrievalBalanced),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &cfg
}
