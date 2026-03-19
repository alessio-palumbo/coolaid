package command

import (
	"fmt"
	"os"

	"ai-cli/internal/llm"
	"ai-cli/internal/prompts"

	"github.com/urfave/cli/v2"
)

func ExplainCommand(llmClient *llm.Client) *cli.Command {
	return &cli.Command{
		Name:  "explain",
		Usage: "explain a source file",
		Action: func(c *cli.Context) error {
			file := c.Args().First()
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}

			prompt, err := prompts.Render(&prompts.Config{Template: prompts.TemplateExplain}, string(data))
			if err != nil {
				return err
			}
			if err := llmClient.GenerateStream(prompt, os.Stdout); err != nil {
				return err
			}

			fmt.Println()
			return nil
		},
	}
}
