package main

import (
	"coolaid/cmd/ai/command"
	"coolaid/cmd/ai/config"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatal(err)
	}

	sw := spinner.NewStreamWriter(os.Stdout)
	client, err := ai.NewClient(cfg, sw)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	app := &cli.App{
		Name:  "ai",
		Usage: "AI powered developer assistant",
		Commands: []*cli.Command{
			command.AskCommand(client, sw),
			command.SummarizeCommand(client, sw),
			command.ExplainCommand(client, sw),
			command.IndexCommand(client),
			command.SearchCommand(client),
			command.QueryCommand(client, sw),
			command.ChatCommand(client, sw),
			command.TestCommand(client, sw),
			command.EditCommand(client, sw),
			command.FixCommand(client, sw),
			command.RefactorCommand(client, sw),
			command.CommentCommand(client, sw),
			command.FlushMemoryCommand(client),
		},
	}

	if err := app.Run(os.Args); err != nil {
		client.Close()
		log.Fatal(err)
	}
}
