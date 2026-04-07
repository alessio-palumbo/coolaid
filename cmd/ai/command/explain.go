package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v2"
)

func ExplainCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:  "explain",
		Usage: "explain a source file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fn",
				Usage: "function to generate test for",
			},
		},
		Action: func(c *cli.Context) error {
			target := ai.Target{
				File:     c.Args().First(),
				Function: c.String("fn"),
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
