package testutil

import (
	"bytes"
	"log/slog"
	"testing"
)

// CaptureLogBuffer redirects the default slog logger to an in-memory buffer and
// restores the original logger in t.Cleanup.
func CaptureLogBuffer(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	originalLogger := slog.Default()
	var logBuf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})
	return &logBuf
}
