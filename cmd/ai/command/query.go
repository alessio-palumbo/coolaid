package command

import (
	"ai-cli/pkg/ai"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func QueryCommand(client *ai.Client) *cli.Command {
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
					ai.RetrievalFast, ai.RetrievalBalanced, ai.RetrievalDeep),
				DefaultText: string(ai.RetrievalFast),
			},
		},
		Action: func(c *cli.Context) error {
			prompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))
			result, err := client.Query(c.Context, prompt, ai.WithRetrievalMode(ai.RetrievalMode(c.String("mode"))))
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
