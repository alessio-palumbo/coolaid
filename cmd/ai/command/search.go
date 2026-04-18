package command

import (
	"context"
	"coolaid/pkg/ai"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

func SearchCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "semantic search in indexed code",
		ArgsUsage: "<prompt>",
		Flags: []cli.Flag{
			kFlag(5),
			mmrFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			prompt := strings.TrimSpace(strings.Join(c.Args().Slice(), " "))

			result, err := client.Search(ctx, prompt, withSearchOptions(c)...)
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
