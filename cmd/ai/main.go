package main

import (
	"ai-cli/cmd/ai/command"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "ai",
		Usage: "AI powered developer assistant",
		Commands: []*cli.Command{
			command.AskCommand(),
			command.SummarizeCommand(),
			command.ExplainCommand(),
			command.IndexCommand(),
			command.SearchCommand(),
		},
	}

	app.Run(os.Args)
}
