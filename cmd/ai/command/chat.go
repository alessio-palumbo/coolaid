package command

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/prompts"
	"ai-cli/internal/vector"
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

func ChatCommand(llmClient *llm.Client, store *vector.Store) *cli.Command {
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

				results, err := embedPromptAndSearch(llmClient, store, input, vector.SearchModeFast)
				if err != nil {
					return err
				}
				prompt, err := prompts.Render(&prompts.Config{Template: prompts.TemplateChat}, input, results...)
				if err != nil {
					return err
				}

				if err := llmClient.ChatStream(prompt, os.Stdout); err != nil {
					return err
				}
				fmt.Println()
			}

			return nil
		},
	}
}
