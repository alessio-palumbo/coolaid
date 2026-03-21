package main

import (
	"ai-cli/cmd/ai/command"
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatal(err)
	}

	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}

	store, err := vector.NewStore(cfg.IndexesDir)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	app := &cli.App{
		Name:  "ai",
		Usage: "AI powered developer assistant",
		Commands: []*cli.Command{
			command.AskCommand(llmClient),
			command.SummarizeCommand(llmClient),
			command.ExplainCommand(llmClient, store),
			command.IndexCommand(llmClient, store, cfg.ConfigDir),
			command.SearchCommand(llmClient, store),
			command.QueryCommand(llmClient, store),
			command.BenchmarkCommand(llmClient, store),
		},
	}

	app.Run(os.Args)
}
