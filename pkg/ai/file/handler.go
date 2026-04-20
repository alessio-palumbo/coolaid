package file

import (
	"context"
	"coolaid/pkg/ai"
)

type FileAppendHandler struct {
	Target ai.Target
}

func (h FileAppendHandler) Handle(ctx context.Context, output string) error {
	out, err := NewCodeOutput(output)
	if err != nil {
		return err
	}

	return out.AppendToFile(h.Target.File)
}
