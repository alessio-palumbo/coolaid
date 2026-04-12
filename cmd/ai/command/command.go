package command

import (
	"coolaid/internal/vector"
	"coolaid/pkg/ai"
	"errors"
	"strings"

	"github.com/urfave/cli/v2"
)

func catchIndexError(err error) error {
	switch {
	case errors.Is(err, vector.ErrNotIndexed):
		return errors.New("project not indexed: run `ai index`")
	case errors.Is(err, vector.ErrReindexRequired):
		return errors.New("index outdated: run `ai index`")
	}
	return err
}

func parseTarget(c *cli.Context) (ai.Target, error) {
	target := ai.Target{
		File:     strings.TrimSpace(c.Args().First()),
		Function: strings.TrimSpace(c.String("fn")),
	}
	if target.File == "" {
		return ai.Target{}, errors.New("missing file argument")
	}
	return target, nil
}
