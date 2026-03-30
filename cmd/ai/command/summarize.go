package command

import (
	"ai-cli/pkg/ai"
	"fmt"

	"github.com/urfave/cli/v2"
)

func SummarizeCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "summarize",
		Usage: "summarize a file",
		Action: func(c *cli.Context) error {
			file := c.Args().First()
			if err := client.Summarize(c.Context, file); err != nil {
				return err
			}

			fmt.Println()
			return nil
		},
	}
}
