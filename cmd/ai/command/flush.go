package command

import (
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func FlushMemoryCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "flush",
		Usage: "flush memory queue",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			sp := spinner.NewSpinner(spinner.WithMessage("Updating memory"))
			processed, err := spinner.Run(sp, os.Stdout, func() (int, error) {
				return client.FlushMemory(c.Context)
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
