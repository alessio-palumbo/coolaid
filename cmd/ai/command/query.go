package command

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/prompts"
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
			query := strings.Join(c.Args().Slice(), " ")
			if query == "" {
				return fmt.Errorf("query required")
			}

			queryVec, err := llmClient.Embed(query)
			if err != nil {
				return err
			}

			results, err := store.SearchForMode(c.String("mode"), queryVec)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No relevant results found")
				return nil
			}

			prompt, err := prompts.Render(
				&prompts.Config{Template: prompts.TemplateQuery, Structured: c.Bool("v")},
				query, results...,
			)
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
