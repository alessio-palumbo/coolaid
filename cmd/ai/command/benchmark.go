package command

import (
	"coolaid/internal/benchmark"
	"coolaid/pkg/ai"

	"github.com/urfave/cli/v2"
)

func BenchmarkCommand(client *ai.Client) *cli.Command {
	return &cli.Command{
		Name:  "benchmark",
		Usage: "benchmark harness to systematically evaluate changes",
		Action: func(c *cli.Context) error {
			benchmark.Run(client)
			return nil
		},
	}
}
