package spinner

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSpinner_StartAndStop(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(
		WithStartDelay(10*time.Millisecond),
		WithFrameInterval(5*time.Millisecond),
		WithMessage("loading"),
		WithFrames([]string{"-", "\\", "|", "/"}),
	)

	stop := s.Start(&buf)

	// allow spinner to render a few frames
	time.Sleep(40 * time.Millisecond)

	stop()

	output := buf.String()

	// Should contain message
	if !strings.Contains(output, "loading") {
		t.Errorf("expected output to contain message, got: %q", output)
	}

	// Should contain at least one frame
	foundFrame := false
	for _, f := range []string{"-", "\\", "|", "/"} {
		if strings.Contains(output, f) {
			foundFrame = true
			break
		}
	}

	if !foundFrame {
		t.Errorf("expected output to contain at least one spinner frame, got: %q", output)
	}
}
