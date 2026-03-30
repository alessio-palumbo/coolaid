package ai

type RetrievalMode string

const (
	RetrievalFast     RetrievalMode = "fast"
	RetrievalBalanced RetrievalMode = "balanced"
	RetrievalDeep     RetrievalMode = "deep"
)

type TaskOption func(*queryConfig)

type queryConfig struct {
	retrieval retrievalOptions
	prompt    promptTaskOptions
}

type retrievalOptions struct {
	k      int
	useMMR bool
}

type promptTaskOptions struct {
	systemOverride   string
	structuredOutput bool
}

func defaultRetrievalOptions(mode RetrievalMode) retrievalOptions {
	switch mode {
	case RetrievalDeep:
		return retrievalOptions{k: 12, useMMR: true}
	case RetrievalBalanced:
		return retrievalOptions{k: 8, useMMR: false}
	default:
		return retrievalOptions{k: 5, useMMR: false}
	}
}

func parseTaskOptions(opts ...TaskOption) *queryConfig {
	cfg := queryConfig{
		retrieval: defaultRetrievalOptions(RetrievalBalanced),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &cfg
}

func WithTopK(k int) TaskOption {
	return func(c *queryConfig) {
		c.retrieval.k = k
	}
}

func WithMMR(enabled bool) TaskOption {
	return func(c *queryConfig) {
		c.retrieval.useMMR = enabled
	}
}

func WithRetrievalMode(mode RetrievalMode) TaskOption {
	return func(c *queryConfig) {
		c.retrieval = defaultRetrievalOptions(mode)
	}
}

func WithSystemPrompt(s string) TaskOption {
	return func(c *queryConfig) {
		c.prompt.systemOverride = s
	}
}

func WithStructuredOutput() TaskOption {
	return func(c *queryConfig) {
		c.prompt.structuredOutput = true
	}
}
