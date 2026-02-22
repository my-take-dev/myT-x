package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRecoverBackgroundPanic(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	t.Run("returns true and logs when panic is recovered", func(t *testing.T) {
		recovered := false
		func() {
			defer func() {
				recovered = recoverBackgroundPanic("worker-a", recover())
			}()
			panic("boom")
		}()

		if !recovered {
			t.Fatal("recoverBackgroundPanic() should return true when recovering panic")
		}
		if !strings.Contains(logBuf.String(), "worker-a") {
			t.Fatalf("log output = %q, want worker name", logBuf.String())
		}
	})

	t.Run("returns false when there is no panic", func(t *testing.T) {
		recovered := true
		func() {
			defer func() {
				recovered = recoverBackgroundPanic("worker-b", recover())
			}()
		}()
		if recovered {
			t.Fatal("recoverBackgroundPanic() should return false when no panic occurred")
		}
	})
}

// NOTE: The retry-loop exhaustion tests (TestNextPanicRestartBackoff,
// TestMaxPanicRestartRetriesLimit) were removed when the inline panic recovery
// loops in startPaneFeedWorker and startIdleMonitor were replaced by
// workerutil.RunWithPanicRecovery. The equivalent tests now live in
// internal/workerutil/recovery_test.go.
