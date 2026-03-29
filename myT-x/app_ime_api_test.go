package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRecoverIMEWindowFocusRaisesWindow(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)

	app := NewApp()
	app.setRuntimeContext(context.Background())

	calls := make([]string, 0, 4)
	runtimeWindowShowFn = func(context.Context) {
		calls = append(calls, "show")
	}
	runtimeWindowUnminimiseFn = func(context.Context) {
		calls = append(calls, "unminimise")
	}
	runtimeWindowSetAlwaysOnTopFn = func(_ context.Context, enabled bool) {
		if enabled {
			calls = append(calls, "always-on-top:true")
			return
		}
		calls = append(calls, "always-on-top:false")
	}

	if err := app.RecoverIMEWindowFocus(); err != nil {
		t.Fatalf("RecoverIMEWindowFocus() error = %v, want nil", err)
	}

	if len(calls) != 4 {
		t.Fatalf("runtime window call count = %d, want 4", len(calls))
	}

	want := []string{"show", "unminimise", "always-on-top:true", "always-on-top:false"}
	for index, expected := range want {
		if calls[index] != expected {
			t.Fatalf("runtime window call %d = %q, want %q", index, calls[index], expected)
		}
	}

	app.windowMu.Lock()
	windowVisible := app.windowVisible
	app.windowMu.Unlock()
	if !windowVisible {
		t.Fatal("windowVisible should be true after RecoverIMEWindowFocus")
	}
}

func TestRecoverIMEWindowFocusSkipsWhenContextNil(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	app := NewApp()
	err := app.RecoverIMEWindowFocus()
	if !errors.Is(err, errRuntimeContextNil) {
		t.Fatalf("RecoverIMEWindowFocus() error = %v, want %v", err, errRuntimeContextNil)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "bringWindowToFront dropped because runtime context is nil") {
		t.Fatalf("log output = %q, want RecoverIMEWindowFocus nil-context warning", logOutput)
	}
}
