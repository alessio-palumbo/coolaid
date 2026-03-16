package command

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func SearchCommand() *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "semantic search in indexed code",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "k",
				Value: 5,
				Usage: "number of results",
			},
		},
		Action: func(c *cli.Context) error {
			prompt := strings.Join(c.Args().Slice(), " ")
			if prompt == "" {
				return fmt.Errorf("query required")
			}

			store, err := vector.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()

			client := llm.NewClient()
			queryVec, err := client.Embed(prompt)
			if err != nil {
				return err
			}

			printResults(store.Search(queryVec, c.Int("k")))
			return nil
		},
	}
}

func printResults(results []vector.Result) {
	for i, r := range results {
		fmt.Printf(
			"\n[%d] score=%.3f\n%s\n\n%s\n",
			i+1,
			r.Score,
			r.Content,
			strings.Repeat("-", 50),
		)
	}
}
