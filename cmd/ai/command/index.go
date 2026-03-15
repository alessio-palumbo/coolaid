package command

import (
	"ai-cli/internal/indexer"
	"ai-cli/internal/llm"
	"fmt"

	"github.com/urfave/cli/v2"
)

func IndexCommand() *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "index the current repository",

		Action: func(c *cli.Context) error {
			store, err := indexer.Build(".", llm.NewClient())
			if err != nil {
				return err
			}

			fmt.Printf("Indexed %d chunks\n", len(store.Items))
			return nil
		},
	}
}
