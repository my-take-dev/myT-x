package sessionlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// capturedEntry holds the arguments received by a test callback.
type capturedEntry struct {
	ts    time.Time
	level slog.Level
	msg   string
	group string
}

// newTestCallback returns a callback that appends captured entries to a slice,
// and a function to retrieve the captured entries.
func newTestCallback() (EntryCallback, func() []capturedEntry) {
	var mu sync.Mutex
	var entries []capturedEntry

	cb := func(ts time.Time, level slog.Level, msg string, group string) {
		mu.Lock()
		defer mu.Unlock()
		entries = append(entries, capturedEntry{
			ts:    ts,
			level: level,
			msg:   msg,
			group: group,
		})
	}

	get := func() []capturedEntry {
		mu.Lock()
		defer mu.Unlock()
		copied := make([]capturedEntry, len(entries))
		copy(copied, entries)
		return copied
	}

	return cb, get
}

func TestTeeHandler_CallsCallbackForErrors(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantMsg string
	}{
		{
			name:    "simple error message",
			msg:     "connection failed",
			wantMsg: "connection failed",
		},
		{
			name:    "error with special characters",
			msg:     "dial tcp 127.0.0.1:5432: connection refused",
			wantMsg: "dial tcp 127.0.0.1:5432: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			cb, getEntries := newTestCallback()

			handler := NewTeeHandler(base, slog.LevelWarn, cb)
			logger := slog.New(handler)

			logger.Error(tt.msg)

			entries := getEntries()
			if len(entries) != 1 {
				t.Fatalf("expected 1 callback entry, got %d", len(entries))
			}

			entry := entries[0]
			if entry.level != slog.LevelError {
				t.Errorf("level = %v, want %v", entry.level, slog.LevelError)
			}
			if entry.msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", entry.msg, tt.wantMsg)
			}
			if entry.group != "" {
				t.Errorf("group = %q, want empty string", entry.group)
			}
			if entry.ts.IsZero() {
				t.Error("timestamp is zero, expected a valid time")
			}
		})
	}
}

func TestTeeHandler_CallsCallbackForWarnings(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantMsg string
	}{
		{
			name:    "simple warning",
			msg:     "disk space low",
			wantMsg: "disk space low",
		},
		{
			name:    "warning with context",
			msg:     "retry attempt 3/5 for upstream",
			wantMsg: "retry attempt 3/5 for upstream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			cb, getEntries := newTestCallback()

			handler := NewTeeHandler(base, slog.LevelWarn, cb)
			logger := slog.New(handler)

			logger.Warn(tt.msg)

			entries := getEntries()
			if len(entries) != 1 {
				t.Fatalf("expected 1 callback entry, got %d", len(entries))
			}

			entry := entries[0]
			if entry.level != slog.LevelWarn {
				t.Errorf("level = %v, want %v", entry.level, slog.LevelWarn)
			}
			if entry.msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", entry.msg, tt.wantMsg)
			}
			if entry.ts.IsZero() {
				t.Error("timestamp is zero, expected a valid time")
			}
		})
	}
}

func TestTeeHandler_IgnoresInfoLevel(t *testing.T) {
	tests := []struct {
		name string
		msg  string
	}{
		{
			name: "info message",
			msg:  "server started on :8080",
		},
		{
			name: "debug message via info logger",
			msg:  "processing request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			cb, getEntries := newTestCallback()

			handler := NewTeeHandler(base, slog.LevelWarn, cb)
			logger := slog.New(handler)

			logger.Info(tt.msg)

			entries := getEntries()
			if len(entries) != 0 {
				t.Fatalf("expected 0 callback entries for Info level, got %d", len(entries))
			}
		})
	}
}

func TestTeeHandler_DelegatesToBase(t *testing.T) {
	tests := []struct {
		name      string
		logFunc   func(logger *slog.Logger)
		wantInBuf string
	}{
		{
			name:      "info reaches base",
			logFunc:   func(l *slog.Logger) { l.Info("info message") },
			wantInBuf: "info message",
		},
		{
			name:      "warn reaches base",
			logFunc:   func(l *slog.Logger) { l.Warn("warn message") },
			wantInBuf: "warn message",
		},
		{
			name:      "error reaches base",
			logFunc:   func(l *slog.Logger) { l.Error("error message") },
			wantInBuf: "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			cb, _ := newTestCallback()

			handler := NewTeeHandler(base, slog.LevelWarn, cb)
			logger := slog.New(handler)

			tt.logFunc(logger)

			output := buf.String()
			if !strings.Contains(output, tt.wantInBuf) {
				t.Errorf("base handler output %q does not contain %q", output, tt.wantInBuf)
			}
		})
	}
}

func TestTeeHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	cb, getEntries := newTestCallback()

	handler := NewTeeHandler(base, slog.LevelWarn, cb)
	grouped := handler.WithGroup("mygroup")
	logger := slog.New(grouped)

	logger.Error("grouped error")

	entries := getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 callback entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.group != "mygroup" {
		t.Errorf("group = %q, want %q", entry.group, "mygroup")
	}
	if entry.level != slog.LevelError {
		t.Errorf("level = %v, want %v", entry.level, slog.LevelError)
	}
	if entry.msg != "grouped error" {
		t.Errorf("msg = %q, want %q", entry.msg, "grouped error")
	}
}

func TestTeeHandler_WithNestedGroups(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	cb, getEntries := newTestCallback()

	handler := NewTeeHandler(base, slog.LevelWarn, cb)
	nested := handler.WithGroup("a").WithGroup("b")
	logger := slog.New(nested)

	logger.Error("nested error")

	entries := getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 callback entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.group != "a.b" {
		t.Errorf("group = %q, want %q", entry.group, "a.b")
	}
	if entry.level != slog.LevelError {
		t.Errorf("level = %v, want %v", entry.level, slog.LevelError)
	}
	if entry.msg != "nested error" {
		t.Errorf("msg = %q, want %q", entry.msg, "nested error")
	}
}

func TestTeeHandler_NilCallback(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	handler := NewTeeHandler(base, slog.LevelWarn, nil)
	logger := slog.New(handler)

	// Should not panic with nil callback.
	logger.Error("should not panic")

	output := buf.String()
	if !strings.Contains(output, "should not panic") {
		t.Errorf("base handler output %q does not contain expected message", output)
	}
}

func TestTeeHandler_WithAttrsPreservesCallback(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	cb, getEntries := newTestCallback()

	handler := NewTeeHandler(base, slog.LevelWarn, cb)
	withAttrs := handler.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(withAttrs)

	logger.Error("attr error")

	entries := getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 callback entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.msg != "attr error" {
		t.Errorf("msg = %q, want %q", entry.msg, "attr error")
	}
	if entry.level != slog.LevelError {
		t.Errorf("level = %v, want %v", entry.level, slog.LevelError)
	}

	// Verify attributes reached the base handler.
	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Errorf("base handler output %q does not contain attribute component=test", output)
	}
}

// errorHandler is a mock [slog.Handler] that always returns a predetermined error
// from Handle. Used to verify TeeHandler behavior when the base handler fails.
type errorHandler struct{ err error }

func (h *errorHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (h *errorHandler) Handle(context.Context, slog.Record) error { return h.err }
func (h *errorHandler) WithAttrs([]slog.Attr) slog.Handler        { return h }
func (h *errorHandler) WithGroup(string) slog.Handler             { return h }

func TestTeeHandler_BaseHandlerError_CallbackStillCalled(t *testing.T) {
	tests := []struct {
		name     string
		baseErr  error
		msg      string
		logLevel slog.Level
	}{
		{
			name:     "disk full error still notifies UI",
			baseErr:  errors.New("disk full"),
			msg:      "critical failure",
			logLevel: slog.LevelError,
		},
		{
			name:     "permission denied error still notifies UI",
			baseErr:  errors.New("permission denied"),
			msg:      "write failed",
			logLevel: slog.LevelWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &errorHandler{err: tt.baseErr}
			cb, getEntries := newTestCallback()

			handler := NewTeeHandler(base, slog.LevelWarn, cb)

			record := slog.NewRecord(time.Now(), tt.logLevel, tt.msg, 0)
			// Intentionally ignore the returned error -- this test verifies that the
			// callback is invoked even when the base handler returns an error.
			_ = handler.Handle(context.Background(), record)

			entries := getEntries()
			if len(entries) != 1 {
				t.Fatalf("expected 1 callback entry even when base errors, got %d", len(entries))
			}

			entry := entries[0]
			if entry.msg != tt.msg {
				t.Errorf("msg = %q, want %q", entry.msg, tt.msg)
			}
			if entry.level != tt.logLevel {
				t.Errorf("level = %v, want %v", entry.level, tt.logLevel)
			}
		})
	}
}

func TestTeeHandler_BaseHandlerError_ErrorPropagated(t *testing.T) {
	tests := []struct {
		name    string
		baseErr error
	}{
		{
			name:    "io error propagated",
			baseErr: errors.New("disk full"),
		},
		{
			name:    "wrapped error propagated",
			baseErr: fmt.Errorf("write log: %w", errors.New("no space left on device")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &errorHandler{err: tt.baseErr}
			cb, _ := newTestCallback()

			handler := NewTeeHandler(base, slog.LevelWarn, cb)

			record := slog.NewRecord(time.Now(), slog.LevelError, "some error", 0)
			err := handler.Handle(context.Background(), record)

			if err == nil {
				t.Fatal("expected error from Handle, got nil")
			}
			if !errors.Is(err, tt.baseErr) {
				t.Errorf("error = %v, want %v", err, tt.baseErr)
			}
		})
	}
}

func TestTeeHandler_WithGroupEmpty(t *testing.T) {
	base := slog.NewTextHandler(io.Discard, nil)
	h := NewTeeHandler(base, slog.LevelInfo, nil)
	result := h.WithGroup("")
	if result != h {
		t.Error("WithGroup(\"\") should return the receiver unchanged")
	}
}

func TestTeeHandler_WithGroupEmpty_PreservesExistingGroup(t *testing.T) {
	var gotGroup string
	base := slog.NewTextHandler(io.Discard, nil)
	h := NewTeeHandler(base, slog.LevelInfo, func(_ time.Time, _ slog.Level, _ string, group string) {
		gotGroup = group
	})
	grouped := h.WithGroup("foo").(*TeeHandler)
	same := grouped.WithGroup("")
	if same != grouped {
		t.Error("WithGroup(\"\") on grouped handler should return receiver unchanged")
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	_ = same.Handle(context.Background(), record)
	if gotGroup != "foo" {
		t.Errorf("expected group 'foo', got %q", gotGroup)
	}
}

func TestTeeHandler_CallbackPanic_DoesNotPropagate(t *testing.T) {
	base := slog.NewTextHandler(io.Discard, nil)
	h := NewTeeHandler(base, slog.LevelInfo, func(_ time.Time, _ slog.Level, _ string, _ string) {
		panic("test panic")
	})
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	// Should not panic.
	err := h.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestTeeHandler_CallbackPanic_WritesToStderr(t *testing.T) {
	origStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = writePipe
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = readPipe.Close()
		_ = writePipe.Close()
	})

	base := slog.NewTextHandler(io.Discard, nil)
	h := NewTeeHandler(base, slog.LevelInfo, func(_ time.Time, _ slog.Level, _ string, _ string) {
		panic("stderr panic test")
	})
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	if handleErr := h.Handle(context.Background(), record); handleErr != nil {
		t.Fatalf("Handle() error = %v, want nil", handleErr)
	}
	_ = writePipe.Close()

	stderrBytes, readErr := io.ReadAll(readPipe)
	if readErr != nil {
		t.Fatalf("io.ReadAll(stderr) error = %v", readErr)
	}
	stderrOutput := string(stderrBytes)
	if !strings.Contains(stderrOutput, "[session-log] callback panicked: stderr panic test") {
		t.Fatalf("stderr output = %q, want panic diagnostic prefix", stderrOutput)
	}
}
