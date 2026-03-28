package command

import (
	"ai-cli/internal/config"
	"ai-cli/internal/indexer"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"fmt"

	"github.com/urfave/cli/v2"
)

func IndexCommand(llmClient *llm.Client, store *vector.Store, cfg *config.Config) *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "index the current repository",

		Action: func(c *cli.Context) error {
			if err := store.Clear(); err != nil {
				return err
			}

			fmt.Println("Indexing project at", store.ProjectRoot)
			if err := indexer.Build(".", store, llmClient, cfg); err != nil {
				return err
			}

			if err := store.Save(); err != nil {
				return err
			}
			fmt.Printf("Indexed %d chunks in %s\n", len(store.Items), store.DBPath)

			return nil
		},
	}
}
