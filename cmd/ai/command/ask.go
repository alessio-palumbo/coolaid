package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	minSearchLimit     = 1
	maxSearchLimit     = 10
	defaultSearchLimit = 5
)

func AskCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:      "ask",
		Usage:     "ask the AI a question",
		ArgsUsage: "<prompt>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "web",
				Usage: "Enable web search",
			},
			&cli.IntFlag{
				Name:  "seach-limit",
				Usage: "The number of search results to retrieve (1-10, default 5)",
				Value: defaultSearchLimit,
			},
		},
		Action: func(c *cli.Context) error {
			prompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))
			opts := ai.AskOptions{
				UseWeb:      c.Bool("web"),
				SearchLimit: withSearchLimit(c.Int("search-limit")),
			}

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

func withSearchLimit(l int) int {
	if l == 0 {
		l = defaultSearchLimit
	}
	return max(minSearchLimit, min(l, maxSearchLimit))
}
