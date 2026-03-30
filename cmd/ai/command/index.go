package command

import (
	"ai-cli/pkg/ai"

	"github.com/urfave/cli/v2"
)

func IndexCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "index the current repository",

		Action: func(c *cli.Context) error {
			return client.Index(c.Context)
		},
	}
}
