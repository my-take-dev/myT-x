package tmux

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestRecoverRouterPanic(t *testing.T) {
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
				recovered = recoverRouterPanic("router-worker", recover())
			}()
			panic("router boom")
		}()

		if !recovered {
			t.Fatal("recoverRouterPanic() should return true when recovering panic")
		}
		if !strings.Contains(logBuf.String(), "router-worker") {
			t.Fatalf("log output = %q, want worker name", logBuf.String())
		}
	})

	t.Run("returns false when there is no panic", func(t *testing.T) {
		recovered := true
		func() {
			defer func() {
				recovered = recoverRouterPanic("router-worker", recover())
			}()
		}()
		if recovered {
			t.Fatal("recoverRouterPanic() should return false when no panic occurred")
		}
	})
}

func TestNextRouterPanicRestartBackoff(t *testing.T) {
	tests := []struct {
		name    string
		current time.Duration
		want    time.Duration
	}{
		{name: "zero uses initial", current: 0, want: initialRouterPanicRestartBackoff},
		{name: "negative uses initial", current: -time.Second, want: initialRouterPanicRestartBackoff},
		{name: "doubles under cap", current: 200 * time.Millisecond, want: 400 * time.Millisecond},
		{name: "caps at max", current: maxRouterPanicRestartBackoff, want: maxRouterPanicRestartBackoff},
		{name: "caps overflow", current: maxRouterPanicRestartBackoff / 2 * 3, want: maxRouterPanicRestartBackoff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRouterPanicRestartBackoff(tt.current); got != tt.want {
				t.Fatalf("nextRouterPanicRestartBackoff(%s) = %s, want %s", tt.current, got, tt.want)
			}
		})
	}
}
