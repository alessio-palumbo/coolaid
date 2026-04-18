package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

func EditCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "edit a file or a function",
		ArgsUsage: "<file> <prompt>",
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

			if c.NArg() < 2 {
				return errors.New("expecting 2 arguments: <file> <prompt>")
			}
			prompt := strings.TrimSpace(strings.Join(c.Args().Tail(), " "))

			return spinner.WrapError(sw, func() error {
				if _, err := client.Edit(ctx, target, prompt, withRagOption(c)...); err != nil {
					return err
				}
				fmt.Println()
				return nil
			})
		},
	}
}
