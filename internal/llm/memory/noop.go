package memory

import (
	"context"
	"io"
)

// noop is a no-op implementation of the Memory interface.
//
// It bypasses all memory-related logic (capture, extraction, persistence)
// while preserving normal command execution and streaming behavior.
//
// This is used when memory is disabled via configuration, allowing callers
// to avoid conditional checks and keep the same code paths.
type noop struct{}

// NewNoop returns a no-op Memory implementation.
//
// It disables all memory behavior (capture, extraction, persistence)
// while preserving normal execution flow. Useful when memory is
// turned off via configuration.
func NewNoop() *noop {
	return &noop{}
}

// Capture executes fn with the provided writer without capturing or storing
// any output. It preserves normal streaming behavior while disabling memory.
func (n *noop) Capture(w io.Writer, in Input, fn func(w io.Writer) error) error {
	return fn(w)
}

// Close is a no-op. There are no background workers or resources to release.
func (n *noop) Close(context.Context) {}
