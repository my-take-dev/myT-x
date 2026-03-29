package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/tmux"
	"myT-x/internal/wsserver"
)

// NOTE: This file overrides package-level function variables
// (runtimeEventsEmitFn) and calls slog.SetDefault to capture log output.
// Both are process-global mutations that are NOT safe for parallel tests.
// Do not use t.Parallel() in any test in this file (I-26).

func TestEmitRuntimeEventWithContextSkipsNilContext(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	eventCount := 0
	runtimeEventsEmitFn = func(context.Context, string, ...any) {
		eventCount++
	}

	app := NewApp()
	app.emitRuntimeEventWithContext(nil, "config:updated", map[string]any{"ok": true})

	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0", eventCount)
	}
	if !strings.Contains(logBuf.String(), "runtime event dropped because app context is nil") {
		t.Fatalf("log output = %q, want nil-context warning", logBuf.String())
	}
}

func TestEmitRuntimeEventWithContextEmitsWhenContextIsReady(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	eventCount := 0
	eventName := ""
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		eventCount++
		eventName = name
	}

	app := NewApp()
	app.emitRuntimeEventWithContext(context.Background(), "config:updated", map[string]any{"ok": true})

	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
	if eventName != "config:updated" {
		t.Fatalf("event name = %q, want %q", eventName, "config:updated")
	}
}

func TestAppRuntimeEventEmitterAdapterEmitUsesRuntimeContext(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	emitCount := 0
	eventName := ""
	var eventCtx context.Context
	runtimeEventsEmitFn = func(ctx context.Context, name string, _ ...any) {
		emitCount++
		eventName = name
		eventCtx = ctx
	}

	app := NewApp()
	appCtx := context.WithValue(context.Background(), struct{ key string }{key: "k"}, "v")
	app.setRuntimeContext(appCtx)
	emitter := newAppRuntimeEventEmitterAdapter(app)

	emitter.Emit("app:input-history-updated", nil)

	if emitCount != 1 {
		t.Fatalf("emit count = %d, want 1", emitCount)
	}
	if eventName != "app:input-history-updated" {
		t.Fatalf("event name = %q, want %q", eventName, "app:input-history-updated")
	}
	if eventCtx != appCtx {
		t.Fatal("adapter did not use app runtime context")
	}
}

func TestAppRuntimeEventEmitterAdapterEmitWithContextUsesExplicitContext(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	emitCount := 0
	var eventCtx context.Context
	eventName := ""
	runtimeEventsEmitFn = func(ctx context.Context, name string, _ ...any) {
		emitCount++
		eventCtx = ctx
		eventName = name
	}

	app := NewApp()
	explicitCtx := context.WithValue(context.Background(), struct{ key string }{key: "explicit"}, "ctx")
	emitter := newAppRuntimeEventEmitterAdapter(app)

	emitter.EmitWithContext(explicitCtx, "worktree:setup-complete", map[string]any{"ok": true})

	if emitCount != 1 {
		t.Fatalf("emit count = %d, want 1", emitCount)
	}
	if eventName != "worktree:setup-complete" {
		t.Fatalf("event name = %q, want %q", eventName, "worktree:setup-complete")
	}
	if eventCtx != explicitCtx {
		t.Fatal("adapter did not use explicit context")
	}
}

func TestNewAppRuntimeEventEmitterAdapterPanicsOnNilApp(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil app")
		}
	}()
	_ = newAppRuntimeEventEmitterAdapter(nil)
}

func TestEmitBackendEventSkipsNilRuntimeContextWithWarning(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	app := NewApp()
	app.emitBackendEvent("app:activate-window", nil)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "backend event dropped because runtime context is nil") {
		t.Fatalf("log output = %q, want backend nil-context warning", logOutput)
	}
	if !strings.Contains(logOutput, "event=app:activate-window") {
		t.Fatalf("log output = %q, want dropped event name", logOutput)
	}
}

// TestEmitBackendEventDelegatesPaneOutputToSnapshotService verifies that
// emitBackendEvent routes "tmux:pane-output" to snapshotService.HandlePaneOutputEvent.
func TestEmitBackendEventDelegatesPaneOutputToSnapshotService(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := NewApp()
	app.setRuntimeContext(context.Background())
	t.Cleanup(func() { app.snapshotService.Shutdown() })

	// Send a valid pane output event through emitBackendEvent.
	app.emitBackendEvent("tmux:pane-output", &tmux.PaneOutputEvent{
		PaneID: "%test",
		Data:   []byte("hello"),
	})

	// Verify that data reached the pane state via the snapshot service pipeline.
	// The service's HandlePaneOutputEvent → enqueuePaneOutput → enqueuePaneStateFeed
	// writes to the feed channel or directly to paneStates.
	// Give the feed channel time to be consumed by checking pane state output.
	app.snapshotService.StartPaneFeedWorker(context.Background())
	defer app.snapshotService.StopPaneFeedWorker()

	waitForCondition(t, 2*time.Second, func() bool {
		return app.paneStates.Snapshot("%test") != ""
	}, "pane output should reach paneStates via snapshotService")
}

// TestEmitBackendEventDebouncesLayoutChangedSnapshots verifies that layout-changed
// events are routed through the snapshot service's debounce mechanism.
func TestEmitBackendEventDebouncesLayoutChangedSnapshots(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var mu sync.Mutex
	snapshotEvents := 0
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		if name != "tmux:snapshot" && name != "tmux:snapshot-delta" {
			return
		}
		mu.Lock()
		snapshotEvents++
		mu.Unlock()
	}

	for range 4 {
		app.emitBackendEvent("tmux:layout-changed", map[string]any{"sessionName": "alpha"})
	}
	waitForCondition(
		t,
		2*time.Second,
		func() bool {
			mu.Lock()
			defer mu.Unlock()
			return snapshotEvents >= 1
		},
		"debounced layout snapshot emission",
	)

	mu.Lock()
	defer mu.Unlock()
	if snapshotEvents != 1 {
		t.Fatalf("snapshot event count = %d, want 1 for debounced layout updates", snapshotEvents)
	}
}

// TestEmitBackendEventPaneOutputLogsTypeForUnknownPayloads verifies that
// unknown payload types are logged through the snapshot service delegation.
// Only the %T type info is logged (not %v) to avoid panic risk from arbitrary String() methods.
func TestEmitBackendEventPaneOutputLogsTypeForUnknownPayloads(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	original := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.emitBackendEvent("tmux:pane-output", struct{ Value string }{Value: "unexpected"})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "[EVENT] pane-output: unexpected payload type") {
		t.Fatalf("missing debug event warning log, output=%q", logOutput)
	}
	if !strings.Contains(logOutput, "type=") {
		t.Fatalf("missing type field in log output=%q", logOutput)
	}
}

// TestGetWebSocketURL verifies the GetWebSocketURL public API (I-7).
func TestGetWebSocketURL(t *testing.T) {
	t.Run("returns empty string when wsHub is nil", func(t *testing.T) {
		app := NewApp()
		got := app.GetWebSocketURL()
		if got != "" {
			t.Fatalf("GetWebSocketURL() = %q, want empty string when wsHub is nil", got)
		}
	})

	t.Run("returns ws URL when wsHub is started", func(t *testing.T) {
		hub := wsserver.NewHub(wsserver.HubOptions{Addr: "127.0.0.1:0"})
		ctx := t.Context()
		if err := hub.Start(ctx); err != nil {
			t.Fatalf("hub.Start() error: %v", err)
		}
		defer func() {
			if err := hub.Stop(); err != nil {
				t.Logf("hub.Stop(): %v", err)
			}
		}()

		app := NewApp()
		app.wsHub = hub

		got := app.GetWebSocketURL()
		if got == "" {
			t.Fatal("GetWebSocketURL() = empty string, want ws:// URL")
		}
		if !strings.HasPrefix(got, "ws://127.0.0.1:") {
			t.Fatalf("GetWebSocketURL() = %q, want ws://127.0.0.1:PORT/ws prefix", got)
		}
		if !strings.HasSuffix(got, "/ws") {
			t.Fatalf("GetWebSocketURL() = %q, want suffix /ws", got)
		}
	})
}
