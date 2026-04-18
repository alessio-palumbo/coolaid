package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

func QueryCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "query",
		Usage:     "ask a question over your indexed code",
		ArgsUsage: "<prompt>",
		Flags: []cli.Flag{
			vFlag(),
			modeFlag(ai.RetrievalFast),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			prompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))

			opts := withModeOption(c)
			if c.Bool("v") {
				opts = append(opts, ai.WithStructuredOutput())
			}

			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Query(ctx, prompt, opts...)
			})
			if err != nil {
				return err
			}

			if result.Status.NoResults {
				fmt.Println("No relevant results found")
			}

			fmt.Println()
			return nil
		},
	}
}
