package command

import (
	"coolaid/internal/store"
	"coolaid/pkg/ai"
	"errors"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"
)

func catchIndexError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotIndexed):
		return errors.New("project not indexed: run `ai index`")
	case errors.Is(err, store.ErrReindexRequired):
		return errors.New("index outdated: run `ai index`")
	}
	return err
}

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
