package spinner

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWriteStopsSpinner(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w, WithStartDelay(1*time.Millisecond), WithFrameInterval(1*time.Millisecond))

	sw.startSpinner()
	time.Sleep(5 * time.Millisecond)

	if _, err := sw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	// spinner should be stopped
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.stopFunc != nil {
		t.Fatal("spinner should be stopped after Write")
	}
}

func TestWrapStopsSpinner(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w, WithStartDelay(1*time.Millisecond), WithFrameInterval(1*time.Millisecond))

	_, err := Wrap(sw, func() (int, error) {
		time.Sleep(10 * time.Millisecond)
		return 42, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sw.stopFunc != nil {
		t.Fatal("spinner should be stopped after Wrap")
	}
}

func TestWrapError(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w, WithStartDelay(1*time.Millisecond), WithFrameInterval(1*time.Millisecond))

	expectedErr := "boom"

	err := WrapError(sw, func() error {
		time.Sleep(5 * time.Millisecond)
		return errors.New(expectedErr)
	})

	if err == nil || err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %v", expectedErr, err)
	}
}

func TestWrapFastFunction_NoSpinner(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w, WithStartDelay(50*time.Millisecond)) // large delay

	_, err := Wrap(sw, func() (int, error) {
		return 1, nil // immediate
	})
	if err != nil {
		t.Fatal(err)
	}

}

func TestOptionsApplied(t *testing.T) {
	w := &mockWriter{}
	frames := []string{"a", "b"}

	sw := NewStreamWriter(w,
		WithStartDelay(123*time.Millisecond),
		WithFrameInterval(456*time.Millisecond),
		WithFrames(frames),
	)

	if sw.spinner.startDelay != 123*time.Millisecond {
		t.Fatal("startDelay not applied")
	}
	if sw.spinner.frameInterval != 456*time.Millisecond {
		t.Fatal("frameInterval not applied")
	}
	if len(sw.spinner.frames) != 2 || sw.spinner.frames[0] != "a" {
		t.Fatal("frames not applied")
	}
}

func TestStartSpinner_Idempotent(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w)

	sw.startSpinner()
	sw.startSpinner() // should not start another

	time.Sleep(5 * time.Millisecond)
	sw.stopSpinner()
}

func TestStopSpinner_Idempotent(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w)

	sw.startSpinner()
	time.Sleep(5 * time.Millisecond)

	sw.stopSpinner()
	sw.stopSpinner() // should not panic
}

func TestSpinnerClearsBeforeWrite(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamWriter(&buf, WithStartDelay(1*time.Millisecond), WithFrameInterval(1*time.Millisecond))

	_ = WrapError(sw, func() error {
		time.Sleep(5 * time.Millisecond)
		_, _ = sw.Write([]byte("done\n"))
		return nil
	})

	out := buf.String()

	if !strings.Contains(out, "done") {
		t.Fatal("expected output to contain 'done'")
	}

	if strings.Contains(out, "Thinking...done") {
		t.Fatal("spinner not cleared before write")
	}
}

func TestSpinnerRestart(t *testing.T) {
	w := &mockWriter{}
	sw := NewStreamWriter(w)

	sw.startSpinner()
	time.Sleep(5 * time.Millisecond)
	sw.stopSpinner()

	sw.startSpinner() // should work again
	time.Sleep(5 * time.Millisecond)
	sw.stopSpinner()
}

type mockWriter struct {
	buf bytes.Buffer
}

func (m *mockWriter) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}
