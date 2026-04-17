package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v3"
)

func SummarizeCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "summarize",
		Usage:     "summarize a file",
		ArgsUsage: "<file>",
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			return spinner.WrapError(sw, func() error {
				if err := client.Summarize(ctx, target.File); err != nil {
					return err
				}
				fmt.Println()
				return nil
			})
		},
	}
}
