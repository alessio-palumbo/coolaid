package command

import (
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func FlushMemoryCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "flush",
		Usage: "flush memory queue",
		Flags: []cli.Flag{},
		Action: func(ctx context.Context, c *cli.Command) error {
			sp := spinner.NewSpinner(spinner.WithMessage("Updating memory"))
			processed, err := spinner.Run(sp, os.Stdout, func() (int, error) {
				return client.FlushMemory(ctx)
			})
			if err != nil {
				return err
			}

			switch processed {
			case 0:
				fmt.Println("✔ No memory updates")
			case 1:
				fmt.Println("✔ Memory updated (1 item)")
			default:
				fmt.Printf("✔ Memory updated (%d items)\n", processed)
			}

			return nil
		},
	}
}
