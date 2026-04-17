package command

import (
	"bufio"
	"context"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

func ChatCommand(client *ai.Client, sw *spinner.StreamWriter) *cli.Command {
	return &cli.Command{
		Name:  "chat",
		Usage: "start a chat with the AI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "mode",
				Value: "fast",
				Usage: fmt.Sprintf("mode determines the algorithm used by RAG [%s, %s, %s]",
					ai.RetrievalFast, ai.RetrievalBalanced, ai.RetrievalDeep),
				DefaultText: string(ai.RetrievalFast),
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			session := client.NewChatSession(ai.WithRetrievalMode(ai.RetrievalMode(c.String("mode"))))
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

				if err := spinner.WrapError(sw, func() error {
					if err := session.Send(ctx, input); err != nil {
						return catchIndexError(err)
					}
					fmt.Println()
					return nil
				}); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
