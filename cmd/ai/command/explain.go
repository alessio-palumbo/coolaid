package command

import (
	"ai-cli/pkg/ai"
	"fmt"

	"github.com/urfave/cli/v2"
)

func ExplainCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "explain",
		Usage: "explain a source file",
		Action: func(c *cli.Context) error {
			file := c.Args().First()
			result, err := client.Explain(c.Context, file)
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
