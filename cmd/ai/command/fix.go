package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v3"
)

func FixCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "fix",
		Usage:     "fix a file or a function",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			fnFlag(),
			rngFlag(),
			ragFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			return spinner.WrapError(sw, func() error {
				if _, err := client.Fix(ctx, target, withRagOption(c)...); err != nil {
					return err
				}
				fmt.Println()
				return nil
			})
		},
	}
}
