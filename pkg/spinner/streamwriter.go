package spinner

import (
	"io"
	"sync"
)

// StreamWriter wraps an io.Writer and coordinates spinner lifecycle
// during long-running operations.
//
// It ensures spinner output does not interleave with normal writes
// by stopping the spinner before writing.
type StreamWriter struct {
	lockedWriter io.Writer
	spinner      *Spinner

	mu       sync.Mutex
	stopFunc func()
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

// NewStreamWriter creates a new StreamWriter wrapping the provided io.Writer.
//
// It also initializes an internal spinner used for long-running operations.
func NewStreamWriter(w io.Writer, opts ...Option) *StreamWriter {
	return &StreamWriter{
		lockedWriter: &lockedWriter{w: w},
		spinner:      NewSpinner(opts...),
	}
}

// Wrap executes fn while displaying a spinner if it takes longer than the configured delay.
//
// The spinner is started before fn executes and stopped immediately after it returns.
//
// fn should focus only on the long-running operation.
func Wrap[T any](sw *StreamWriter, fn func() (T, error)) (T, error) {
	sw.startSpinner()
	t, err := fn()
	sw.stopSpinner()
	return t, err
}

// WrapError is a convenience variant of Wrap for functions that return only an error.
func WrapError(sw *StreamWriter, fn func() error) error {
	sw.startSpinner()
	err := fn()
	sw.stopSpinner()
	return err
}

// Write stops the spinner (if running) and writes to the underlying writer.
//
// This ensures spinner output does not interleave with user output.
func (sw *StreamWriter) Write(p []byte) (int, error) {
	sw.stopSpinner()
	return sw.lockedWriter.Write(p)
}

// startSpinner starts the spinner if it is not already running.
//
// It is a no-op if the spinner is already active.
func (sw *StreamWriter) startSpinner() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// If stopFunc is already set, a goroutine is already active
	if sw.stopFunc != nil {
		return
	}
	sw.stopFunc = sw.spinner.Start(sw.lockedWriter)
}

// stopSpinner stops the spinner if it is running and waits for it to exit.
//
// It guarantees that spinner output is cleared before subsequent writes.
func (sw *StreamWriter) stopSpinner() {
	sw.mu.Lock()
	if sw.stopFunc == nil {
		sw.mu.Unlock()
		return
	}
	stop := sw.stopFunc
	sw.mu.Unlock()

	stop()

	sw.mu.Lock()
	sw.stopFunc = nil
	sw.mu.Unlock()
}
