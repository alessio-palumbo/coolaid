package command

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
)

func embedPromptAndSearch(llmClient *llm.Client, store *vector.Store, prompt string, queryMode string) ([]vector.Result, error) {
	queryVec, err := llmClient.Embed(prompt)
	if err != nil {
		return nil, err
	}
	return store.SearchForMode(queryMode, queryVec)
}

func enrichWithSummary(prompt, summary string) string {
	return prompt + "\n\n" + summary
}

func shouldRetry(results []vector.Result) bool {
	return len(results) == 0 || results[0].Score < 0.5
}
