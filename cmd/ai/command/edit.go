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
		Action: func(ctx context.Context, c *cli.Command) error {
			target, err := parseTarget(c)
			if err != nil {
				return err
			}

			if c.NArg() < 2 {
				return errors.New("expecting 2 arguments: <file> <prompt>")
			}
			prompt := strings.TrimSpace(strings.Join(c.Args().Tail(), " "))

			ragMode := ai.RetrievalNone
			if c.Bool("rag") {
				ragMode = ai.RetrievalBalanced
			}
			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.Edit(ctx, target, prompt, ai.WithRetrievalMode(ragMode))
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
