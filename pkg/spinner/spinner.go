// Package spinner provides a lightweight terminal spinner that wraps an io.Writer.
//
// It is designed to give visual feedback during long-running operations,
// such as LLM requests, without coupling to any specific domain or CLI framework.
//
// The spinner starts after a configurable delay and stops automatically
// when output is written through the StreamWriter or when the wrapped
// operation completes.
package spinner

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	defaultStartDelay    = 200 * time.Millisecond
	defaultFrameInterval = 100 * time.Millisecond
)

var (
	defaultMessage = " Thinking"
	defaultFrames  = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// Option configures a StreamWriter.
type Option func(sw *StreamWriter)

// WithStartDelay sets how long to wait before starting the spinner.
// This avoids flickering for fast operations.
func WithStartDelay(d time.Duration) Option {
	return func(sw *StreamWriter) {
		sw.startDelay = d
	}
}

// WithFrameInterval sets the delay between spinner frames.
func WithFrameInterval(d time.Duration) Option {
	return func(sw *StreamWriter) {
		sw.frameInterval = d
	}
}

// WithMessage sets the message used for the spinner.
// If empty, a default set is used.
func WithMessage(msg string) Option {
	return func(sw *StreamWriter) {
		sw.message = msg
	}
}

// WithFrames sets the characters used for the spinner animation.
// If empty, a default set is used.
func WithFrames(frames []string) Option {
	return func(sw *StreamWriter) {
		sw.frames = frames
	}
}

// StreamWriter wraps an io.Writer and displays a spinner while waiting for output.
//
// The spinner starts after a delay and stops automatically when data is written
// through this writer or when the associated operation completes.
//
// A StreamWriter is safe for sequential use but is not intended for concurrent
// use across multiple independent operations.
type StreamWriter struct {
	w io.Writer

	message       string
	frames        []string
	startDelay    time.Duration
	frameInterval time.Duration

	writeMu sync.Mutex

	wg     sync.WaitGroup
	mu     sync.Mutex
	cancel context.CancelFunc
}

// New creates a new StreamWriter wrapping the provided io.Writer.
//
// Optional configuration can be provided via functional options.
// Sensible defaults are applied for delay, frame interval, message,
// and frames when not explicitly set.
func New(w io.Writer, opts ...Option) *StreamWriter {
	sw := &StreamWriter{
		w:             w,
		startDelay:    defaultStartDelay,
		frameInterval: defaultFrameInterval,
	}

	for _, o := range opts {
		o(sw)
	}

	if sw.message == "" {
		sw.message = defaultMessage
	}
	if len(sw.frames) == 0 {
		sw.frames = defaultFrames
	}

	return sw
}

// Wrap executes fn while displaying a spinner if it takes longer than the configured delay.
//
// The spinner is intended to wrap a single long-running operation (e.g. a network or LLM call).
// It starts after the configured delay and stops automatically when fn returns or when output
// is written to the StreamWriter.
//
// fn should focus on the slow operation itself. Additional logic (e.g. printing results)
// is typically better handled outside the wrapper.
func Wrap[T any](sw *StreamWriter, fn func() (T, error)) (T, error) {
	timer := time.AfterFunc(sw.startDelay, sw.startSpinner)
	defer timer.Stop()

	t, err := fn()
	sw.stopSpinner()
	return t, err
}

// WrapError is a convenience variant of Wrap for functions that return only an error.
//
// It follows the same usage guidelines as Wrap: fn should represent a single long-running
// operation. Avoid placing unrelated logic inside fn; keep it focused on the work that
// benefits from spinner feedback.
func WrapError(sw *StreamWriter, fn func() error) error {
	timer := time.AfterFunc(sw.startDelay, sw.startSpinner)
	defer timer.Stop()

	err := fn()
	sw.stopSpinner()
	return err
}

// Write writes to the underlying writer and stops the spinner if it is running.
//
// This ensures that spinner output does not interleave with user output.
func (sw *StreamWriter) Write(p []byte) (int, error) {
	sw.stopSpinner()

	sw.writeMu.Lock()
	defer sw.writeMu.Unlock()
	return sw.w.Write(p)
}

// startSpinner starts the spinner goroutine if one is not already running.
//
// If a spinner is already active (or shutting down), this call is a no-op.
// The spinner runs until its context is cancelled via stopSpinner.
func (sw *StreamWriter) startSpinner() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// If cancel is already set, a goroutine is already active
	if sw.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	sw.cancel = cancel

	sw.wg.Add(1)
	go func(ctx context.Context) {
		defer sw.wg.Done()
		i := 0

		ticker := time.NewTicker(sw.frameInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Move the cursor to the start of the line and clear it.
				sw.spinnerWrite("\r\033[K")
				return
			case <-ticker.C:
				sw.spinnerWrite("\r" + sw.frames[i%len(sw.frames)] + " " + sw.message)
				i = (i + 1) % len(sw.frames)
			}
		}
	}(ctx)
}

// stopSpinner stops the spinner if it is running and waits for the
// spinner goroutine to exit before returning.
//
// This guarantees that any spinner output is fully cleared before
// subsequent writes occur.
func (sw *StreamWriter) stopSpinner() {
	sw.mu.Lock()
	if sw.cancel == nil {
		sw.mu.Unlock()
		return
	}
	cancel := sw.cancel
	sw.mu.Unlock()

	cancel()
	sw.wg.Wait()

	sw.mu.Lock()
	sw.cancel = nil
	sw.mu.Unlock()
}

func (sw *StreamWriter) spinnerWrite(s string) {
	sw.writeMu.Lock()
	defer sw.writeMu.Unlock()
	fmt.Fprint(sw.w, s)
}
