package command

import (
	"ai-cli/pkg/ai"
	"fmt"

	"github.com/urfave/cli/v2"
)

func TestCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "test",
		Usage: "generate tests for a file or function",
		Action: func(c *cli.Context) error {
			target := c.Args().First()
			result, err := client.GenerateTests(c.Context, target, ai.WithRetrievalMode(ai.RetrievalBalanced))
			if err != nil {
				return err
			}
			if result.Status.NoResults {
				fmt.Println("No relevant results found")
			}

			return nil
		},
	}
}
