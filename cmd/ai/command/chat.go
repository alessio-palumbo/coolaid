package command

import (
	"ai-cli/pkg/ai"
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

func ChatCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "chat",
		Usage: "start a chat with the AI",
		Action: func(c *cli.Context) error {
			fmt.Println("Starting AI chat. Type 'exit' or Ctrl+C to quit.")

			reader := bufio.NewReader(os.Stdin)

			for {
				fmt.Print("\n> ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)

				if input == "" {
					continue
				}
				if input == "exit" {
					break
				}

				if err := client.Chat(c.Context, input, ai.WithRetrievalMode(ai.RetrievalFast)); err != nil {
					return err
				}
				fmt.Println()
			}

			return nil
		},
	}
}
