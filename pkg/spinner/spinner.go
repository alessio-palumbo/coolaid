// Package spinner provides a lightweight terminal spinner for long-running operations.
//
// It renders animated feedback to an io.Writer after a configurable delay and
// stops via a cancellation function returned from Start.
//
// The spinner is UI-only and has no knowledge of StreamWriter or any I/O policies.
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

// Option configures a Spinner.
type Option func(s *Spinner)

// WithStartDelay sets how long to wait before starting the spinner.
// This avoids flickering for fast operations.
func WithStartDelay(d time.Duration) Option {
	return func(s *Spinner) {
		s.startDelay = d
	}
}

// WithFrameInterval sets the delay between spinner frames.
func WithFrameInterval(d time.Duration) Option {
	return func(s *Spinner) {
		s.frameInterval = d
	}
}

// WithMessage sets the message used for the spinner.
// If empty, a default set is used.
func WithMessage(msg string) Option {
	return func(s *Spinner) {
		s.message = msg
	}
}

// WithFrames sets the characters used for the spinner animation.
// If empty, a default set is used.
func WithFrames(frames []string) Option {
	return func(s *Spinner) {
		s.frames = frames
	}
}

// Spinner renders an animated terminal indicator to an io.Writer.
//
// It is independent of any higher-level I/O coordination and is intended
// to be controlled via Start/StopFunc.
type Spinner struct {
	message       string
	frames        []string
	startDelay    time.Duration
	frameInterval time.Duration
}

// StopFunc stops a running spinner and waits for it to fully exit.
//
// It is safe to call multiple times, but only the first call has effect.
type StopFunc func()

// NewSpinner creates a new Spinner with optional configuration.
//
// If no options are provided, sensible defaults are used for:
//   - start delay
//   - frame interval
//   - animation frames
//   - message
func NewSpinner(opts ...Option) *Spinner {
	s := &Spinner{
		startDelay:    defaultStartDelay,
		frameInterval: defaultFrameInterval,
		message:       defaultMessage,
		frames:        defaultFrames,
	}

	for _, o := range opts {
		o(s)
	}
	if len(s.frames) == 0 {
		s.frames = defaultFrames
	}

	return s
}

// Start begins rendering the spinner to the provided writer.
//
// The spinner starts after the configured delay and continues until the returned
// StopFunc is called.
//
// It is safe to call StopFunc multiple times, but only the first call has effect.
//
// The caller is responsible for ensuring the writer is safe for concurrent use.
func (s *Spinner) Start(w io.Writer) StopFunc {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	go func(ctx context.Context) {
		defer wg.Done()
		i := 0

		timer := time.NewTimer(s.startDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		timer.Stop()

		ticker := time.NewTicker(s.frameInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Fprint(w, "\r\033[K")
				return

			case <-ticker.C:
				fmt.Fprintf(w, "\r%s %s", s.frames[i%len(s.frames)], s.message)
				i = (i + 1) % len(s.frames)
			}
		}
	}(ctx)

	return func() {
		cancel()
		wg.Wait()
	}
}

// Run executes fn while displaying the spinner provided by s.
//
// The spinner is started before fn is executed and automatically stopped
// after fn returns (successfully or with an error).
//
// This helper is a convenience wrapper around Start/StopFunc and is useful
// when you want to temporarily decorate a long-running operation with
// terminal feedback without manually managing spinner lifecycle.
func Run[T any](s *Spinner, w io.Writer, fn func() (T, error)) (T, error) {
	stop := s.Start(w)
	defer stop()
	return fn()
}
