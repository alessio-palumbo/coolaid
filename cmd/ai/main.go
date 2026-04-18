package main

import (
	"context"
	"coolaid/cmd/ai/command"
	"coolaid/cmd/ai/config"
	"coolaid/internal/store"
	"coolaid/pkg/ai"
	"coolaid/pkg/spinner"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	cmd := &cli.Command{
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

	if err := cmd.Run(ctx, os.Args); err != nil {
		client.Close()
		log.Fatal(catchIndexError(err))
	}
}

func catchIndexError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotIndexed):
		return errors.New("project not indexed: run `ai index`")
	case errors.Is(err, store.ErrReindexRequired):
		return errors.New("index outdated: run `ai index`")
	}
	return err
}
