package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v3"
)

func CommentCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "comment",
		Usage:     "comment a file or a function",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fn",
				Usage: "function to edit",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Comment(ctx, target, ai.WithRetrievalMode(ai.RetrievalNone))
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
