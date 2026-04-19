package command

import (
	"coolaid/pkg/ai"
	"errors"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"
)

const (
	minSearchLimit = 1
	maxSearchLimit = 10
)

func parseTarget(c *cli.Command) (ai.Target, error) {
	target := ai.Target{
		File:     strings.TrimSpace(c.Args().First()),
		Function: strings.TrimSpace(c.String("fn")),
	}

	if target.File == "" {
		return ai.Target{}, errors.New("missing file argument")
	}

	rng := strings.TrimSpace(c.String("rng"))
	if rng != "" {
		var start, end int
		var err error

		if strings.Contains(rng, "-") {
			parts := strings.Split(rng, "-")
			if len(parts) != 2 {
				return ai.Target{}, errors.New("invalid range format, use start-end")
			}
			start, err = strconv.Atoi(parts[0])
			if err != nil {
				return ai.Target{}, err
			}
			end, err = strconv.Atoi(parts[1])
			if err != nil {
				return ai.Target{}, err
			}
		} else {
			return ai.Target{}, errors.New("invalid range format, use start:end or start-end")
		}

		target.StartLine = start
		target.EndLine = end
	}

	return target, nil
}

func withModeOption(c *cli.Command) []ai.TaskOption {
	return []ai.TaskOption{ai.WithRetrievalMode(ai.RetrievalMode(c.String("mode")))}
}

func withRagOption(c *cli.Command) []ai.TaskOption {
	if !c.Bool("rag") {
		return []ai.TaskOption{ai.WithNoRetrieval()}
	}
	return nil
}

func withWebOption(c *cli.Command) []ai.TaskOption {
	if l := c.Int("web"); l > 0 {
		return []ai.TaskOption{ai.WithWebSearch(max(minSearchLimit, min(l, maxSearchLimit)))}
	}
	return nil
}

func withSearchOptions(c *cli.Command) []ai.TaskOption {
	return []ai.TaskOption{ai.WithTopK(c.Int("k")), ai.WithMMR(c.Bool("mmr"))}
}
