package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v3"
)

func ExplainCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "explain a source file",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			fnFlag(),
			rngFlag(),
			modeFlag(ai.RetrievalBalanced),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			return spinner.WrapError(sw, func() error {
				if _, err := client.Explain(ctx, target, withModeOption(c)...); err != nil {
					return err
				}
				fmt.Println()
				return nil
			})
		},
	}
}
