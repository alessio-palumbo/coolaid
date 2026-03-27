package command

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"ai-cli/internal/llm"
	"ai-cli/internal/prompts"
	"ai-cli/internal/query"
	"ai-cli/internal/vector"

	"github.com/urfave/cli/v2"
)

func ExplainCommand(llmClient *llm.Client, store *vector.Store) *cli.Command {
	return &cli.Command{
		Name:  "explain",
		Usage: "explain a source file",
		Action: func(c *cli.Context) error {
			file := c.Args().First()
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}

			// Find dependencies chunks to pass as dependencies to LLM.
			signals := query.ExtractSignals(file, data)
			embedding, err := llmClient.Embed(signals)
			if err != nil {
				return err
			}
			results, err := store.Search(embedding, 8, false)
			if err != nil {
				return err
			}

			// Exclude any chunks matching the file to avoid wasting tokens.
			for i, r := range results {
				if strings.Contains(r.FilePath, file) {
					results = slices.Delete(results, i, i+1)
				}
			}

			content := fmt.Sprintf("file: %s\n\n", file) + string(data)
			prompt, err := prompts.Render(&prompts.Config{Template: prompts.TemplateExplain}, content, results...)
			if err != nil {
				return err
			}
			if err := llmClient.GenerateStream(prompt, os.Stdout); err != nil {
				return err
			}

			fmt.Println()
			return nil
		},
	}
}
