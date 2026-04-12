package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v2"
)

func FixCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "fix",
		Usage:     "fix a file or a function",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "fn",
				Usage: "function to edit",
			},
			&cli.StringFlag{
				Name:  "rng",
				Usage: "start and end line to edit (start-end)",
			},
			&cli.BoolFlag{
				Name:  "rag",
				Value: false,
				Usage: "use RAG for more context",
			},
		},
		Action: func(c *cli.Context) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			ragMode := ai.RetrievalNone
			if c.Bool("rag") {
				ragMode = ai.RetrievalBalanced
			}
			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Fix(c.Context, target, ai.WithRetrievalMode(ragMode))
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
