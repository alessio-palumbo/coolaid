package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func AskCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:  "ask",
		Usage: "ask the AI a question",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "web",
				Usage: "Enable web search",
			},
		},
		Action: func(c *cli.Context) error {
			prompt := strings.Join(c.Args().Slice(), " ")
			opts := ai.AskOptions{UseWeb: c.Bool("web")}

			return spinner.WrapError(sw, func() error {
				if err := client.Ask(c.Context, prompt, opts); err != nil {
					return catchIndexError(err)
				}
				fmt.Println()
				return nil
			})
		},
	}
}
