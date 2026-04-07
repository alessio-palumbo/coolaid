package command

import (
	"coolaid/pkg/ai"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func SearchCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "search",
		Usage: "semantic search in indexed code",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "k",
				Value: 5,
				Usage: "number of results",
			},
			&cli.BoolFlag{
				Name:  "mmr",
				Value: false,
				Usage: "use Max Marginal Relevance",
			},
		},
		Action: func(c *cli.Context) error {
			prompt := strings.Join(c.Args().Slice(), " ")

			result, err := client.Search(c.Context, prompt, ai.WithTopK(c.Int("k")), ai.WithMMR(c.Bool("mmr")))
			if err != nil {
				return catchIndexError(err)
			}

			if result.Status.NoResults {
				fmt.Println("No relevant results found")
			}

			fmt.Println()
			return nil
		},
	}
}
