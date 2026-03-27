package command

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/prompts"
	"ai-cli/internal/query"
	"ai-cli/internal/vector"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

func QueryCommand(llmClient *llm.Client, store *vector.Store) *cli.Command {
	return &cli.Command{
		Name:  "query",
		Usage: "ask a question over your indexed code",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "v",
				Value: false,
				Usage: "set to true for verbose structure output",
			},
			&cli.StringFlag{
				Name:  "mode",
				Value: "fast",
				Usage: fmt.Sprintf("query mode determine the algorithm used by RAG [%s, %s, %s]",
					vector.SearchModeFast, vector.SearchModeBalanced, vector.SearchModeDeep),
				DefaultText: string(vector.SearchModeFast),
			},
		},
		Action: func(c *cli.Context) error {
			originalPrompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))
			if originalPrompt == "" {
				return fmt.Errorf("prompt required")
			}

			queryMode := c.String("mode")

			searchPrompt := originalPrompt
			var usedSummary bool
			if !query.IsSearchable(searchPrompt) {
				// Make sure the Summary is present before appending.
				store.EnsureLoaded()
				searchPrompt = enrichWithSummary(searchPrompt, store.Summary)
				usedSummary = true
			}

			results, err := embedPromptAndSearch(llmClient, store, searchPrompt, queryMode)
			if err != nil {
				return err
			}

			if shouldRetry(results) && !usedSummary {
				searchPrompt = enrichWithSummary(searchPrompt, store.Summary)
				usedSummary = true

				results, err = embedPromptAndSearch(llmClient, store, searchPrompt, queryMode)
				if err != nil {
					return err
				}
			}
			if len(results) == 0 {
				fmt.Println("No relevant results found")
				return nil
			}

			pConfig := &prompts.Config{Template: prompts.TemplateQuery, Structured: c.Bool("v")}
			if usedSummary {
				pConfig.Summary = store.Summary
			}
			renderedPrompt, err := prompts.Render(pConfig, originalPrompt, results...)
			if err != nil {
				return err
			}

			if err := llmClient.GenerateStream(renderedPrompt, os.Stdout); err != nil {
				return err
			}

			fmt.Println()
			return nil
		},
	}
}
