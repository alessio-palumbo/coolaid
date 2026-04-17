package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v3"
)

func TestCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "test",
		Usage:     "generate tests for a file or function",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fn",
				Usage: "function to generate test for",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.GenerateTests(ctx, target, ai.WithRetrievalMode(ai.RetrievalBalanced))
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
