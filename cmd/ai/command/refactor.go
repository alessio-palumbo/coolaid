package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v3"
)

func RefactorCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "refactor",
		Usage:     "refactor a file or a function",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fn",
				Usage: "function to edit",
			},
			&cli.StringFlag{
				Name:  "rng",
				Usage: "start and end line to edit (start-end)",
			},
			&cli.BoolFlag{
				Name:  "rag",
				Value: false,
				Usage: "use RAG for more context",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			ragMode := ai.RetrievalNone
			if c.Bool("rag") {
				ragMode = ai.RetrievalBalanced
			}
			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Refactor(ctx, target, ai.WithRetrievalMode(ragMode))
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
