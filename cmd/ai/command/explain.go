package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v2"
)

func ExplainCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "explain a source file",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fn",
				Usage: "function to generate test for",
			},
		},
		Action: func(c *cli.Context) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Explain(c.Context, target)
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
