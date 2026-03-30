package command

import (
	"ai-cli/pkg/ai"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func AskCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "ask",
		Usage: "ask the AI a question",
		Action: func(c *cli.Context) error {
			prompt := strings.Join(c.Args().Slice(), " ")
			if err := client.Ask(c.Context, prompt); err != nil {
				return err
			}

			fmt.Println()
			return nil
		},
	}
}
