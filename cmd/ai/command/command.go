package command

import (
	"ai-cli/internal/vector"
	"errors"
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
