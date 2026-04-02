package command

import (
	"ai-cli/internal/vector"
	"ai-cli/pkg/ai"
	"errors"
	"strings"
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

// parseTarget splits input like "file.go:FuncName" into file path and function name
// and returns a ai.Target.
func parseTarget(arg string) ai.Target {
	parts := strings.SplitN(arg, ":", 2)
	t := ai.Target{File: parts[0]}
	if len(parts) == 2 {
		t.Function = strings.TrimSpace(parts[1])
	}
	return t
}
