package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func QueryCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
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
				Usage: fmt.Sprintf("mode determines the algorithm used by RAG [%s, %s, %s]",
					ai.RetrievalFast, ai.RetrievalBalanced, ai.RetrievalDeep),
				DefaultText: string(ai.RetrievalFast),
			},
		},
		Action: func(c *cli.Context) error {
			prompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))
			opts := []ai.TaskOption{ai.WithRetrievalMode(ai.RetrievalMode(c.String("mode")))}
			if c.Bool("v") {
				opts = append(opts, ai.WithStructuredOutput())
			}

			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Query(c.Context, prompt, opts...)
			})
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
