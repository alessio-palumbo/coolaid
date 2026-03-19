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

type queryMode string

const (
	ModeFast     queryMode = "fast"
	ModeBalanced queryMode = "balanced"
	ModeDeep     queryMode = "deep"
)

func QueryCommand(llmClient *llm.Client, store *vector.Store) *cli.Command {
	return &cli.Command{
		Name:  "query",
		Usage: "ask a question over your indexed code",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "mode",
				Value: "fast",
				Usage: "query mode determine the algorithm used by RAG",
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

			results, err := searchByMode(c, store, queryVec)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No relevant results found")
				return nil
			}

			prompt, err := prompts.Render(prompts.TemplateQuery, query, results...)
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

func searchByMode(c *cli.Context, store *vector.Store, queryVec []float64) ([]vector.Result, error) {
	switch queryMode(c.String("mode")) {
	case ModeDeep:
		return store.SearchMMR(queryVec, 12, 0.85)
	case ModeBalanced:
		return store.Search(queryVec, 8)
	default:
		return store.Search(queryVec, 5)
	}
}
