package memory

import (
	"io"
	"strings"
)

type teeWriter struct {
	w   io.Writer
	buf strings.Builder
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.buf.Write(p)
	return t.w.Write(p)
}

func (t *teeWriter) String() string {
	return t.buf.String()
}
