package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

func AskCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "ask",
		Usage:     "ask the AI a question",
		ArgsUsage: "<prompt>",
		Flags: []cli.Flag{
			webFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			prompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))

			return spinner.WrapError(sw, func() error {
				if err := client.Ask(ctx, prompt, withWebOptions(c)); err != nil {
					return err
				}
				fmt.Println()
				return nil
			})
		},
	}
}
