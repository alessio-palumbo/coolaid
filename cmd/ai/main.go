package main

import (
	"ai-cli/cmd/ai/command"
	"ai-cli/cmd/ai/config"
	"ai-cli/pkg/ai"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatal(err)
	}

	client, err := ai.NewClient(cfg, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	app := &cli.App{
		Name:  "ai",
		Usage: "AI powered developer assistant",
		Commands: []*cli.Command{
			command.AskCommand(client),
			command.SummarizeCommand(client),
			command.ExplainCommand(client),
			command.IndexCommand(client),
			command.SearchCommand(client),
			command.QueryCommand(client),
			command.ChatCommand(client),
			command.TestCommand(client),
			command.BenchmarkCommand(client),
		},
	}

	app.Run(os.Args)
}
