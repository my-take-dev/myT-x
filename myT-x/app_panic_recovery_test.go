package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
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

func TestNextPanicRestartBackoff(t *testing.T) {
	tests := []struct {
		name    string
		current time.Duration
		want    time.Duration
	}{
		{name: "zero uses initial", current: 0, want: initialPanicRestartBackoff},
		{name: "negative uses initial", current: -time.Second, want: initialPanicRestartBackoff},
		{name: "doubles under cap", current: 200 * time.Millisecond, want: 400 * time.Millisecond},
		{name: "caps at max", current: maxPanicRestartBackoff, want: maxPanicRestartBackoff},
		{name: "caps overflow", current: maxPanicRestartBackoff / 2 * 3, want: maxPanicRestartBackoff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPanicRestartBackoff(tt.current); got != tt.want {
				t.Fatalf("nextPanicRestartBackoff(%s) = %s, want %s", tt.current, got, tt.want)
			}
		})
	}
}
