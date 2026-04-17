package command

import (
	"context"
	"coolaid/pkg/ai"
	"fmt"

	"github.com/urfave/cli/v3"
)

func IndexCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "index the current repository",
		Action: func(ctx context.Context, c *cli.Command) error {
			fmt.Println("Indexing project at", client.ProjectRoot())

			onProgress := func(p ai.IndexProgress) {
				fmt.Printf("\r%-*s", 150, fmt.Sprintf("Indexing files: %d/%d (file: %s [%d bytes])", p.Done, p.Total, p.File, p.Size))
			}
			onComplete := func(r ai.IndexResult) {
				fmt.Printf("\nIndexed %d chunks in %s [elapsed: %.1fs]\n", r.Chunks, r.StoreLocation, float64(r.Elapsed.Milliseconds())/1000)
			}
			return client.Index(ctx, onProgress, onComplete)

		},
	}
}
