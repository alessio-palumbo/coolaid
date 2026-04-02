package command

import (
	"ai-cli/pkg/ai"
	"ai-cli/pkg/spinner"
	"fmt"

	"github.com/urfave/cli/v2"
)

func TestCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:  "test",
		Usage: "generate tests for a file or function",
		Action: func(c *cli.Context) error {
			target := parseTarget(c.Args().First())

			result, err := spinner.Wrap(sw, func() (ai.TaskResult, error) {
				return client.GenerateTests(c.Context, target, ai.WithRetrievalMode(ai.RetrievalBalanced))
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
