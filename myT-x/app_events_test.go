package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"myT-x/internal/panestate"
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
	// I-26: slog.SetDefault mutates process-global state; t.Parallel() is
	// intentionally NOT used in this file to avoid concurrent logger conflicts.
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

func TestEmitBackendEventPaneOutputAcceptsKnownPayloadTypes(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	tests := []struct {
		name       string
		payload    any
		wantPaneID string
		wantData   string
	}{
		{
			name: "value payload",
			payload: tmux.PaneOutputEvent{
				PaneID: " %1 ",
				Data:   []byte("abc"),
			},
			wantPaneID: "%1",
			wantData:   "abc",
		},
		{
			name: "pointer payload",
			payload: &tmux.PaneOutputEvent{
				PaneID: "%2",
				Data:   []byte("xyz"),
			},
			wantPaneID: "%2",
			wantData:   "xyz",
		},
		{
			name: "map payload",
			payload: map[string]any{
				"paneId": " %3 ",
				"data":   "hello",
			},
			wantPaneID: "%3",
			wantData:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.setRuntimeContext(context.Background())

			app.emitBackendEvent("tmux:pane-output", tt.payload)
			defer app.stopOutputBuffer(tt.wantPaneID)

			select {
			case item := <-app.paneFeedCh:
				if item.paneID != tt.wantPaneID {
					t.Fatalf("paneID = %q, want %q", item.paneID, tt.wantPaneID)
				}
				if got := string(item.chunk); got != tt.wantData {
					t.Fatalf("chunk = %q, want %q", got, tt.wantData)
				}
				putFeedBuffer(item.poolPtr)
			default:
				t.Fatal("pane output was not enqueued")
			}
		})
	}
}

func TestEmitBackendEventPaneOutputRejectsUnknownPayloads(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	tests := []struct {
		name    string
		payload any
	}{
		{
			name:    "nil pane output pointer",
			payload: (*tmux.PaneOutputEvent)(nil),
		},
		{
			name: "missing pane id in map payload",
			payload: map[string]any{
				"data": "hello",
			},
		},
		{
			name:    "unsupported payload type",
			payload: struct{ Value string }{Value: "unexpected"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.setRuntimeContext(context.Background())

			app.emitBackendEvent("tmux:pane-output", tt.payload)

			select {
			case item := <-app.paneFeedCh:
				putFeedBuffer(item.poolPtr)
				t.Fatalf("unexpected pane output enqueue: %+v", item)
			default:
			}
		})
	}
}

func TestPaneOutputEventFromPayload(t *testing.T) {
	t.Run("returns same pointer for pointer payload", func(t *testing.T) {
		input := &tmux.PaneOutputEvent{PaneID: "%1", Data: []byte("abc")}
		got, ok := paneOutputEventFromPayload(input)
		if !ok {
			t.Fatal("paneOutputEventFromPayload(pointer) should be handled")
		}
		if got != input {
			t.Fatalf("paneOutputEventFromPayload(pointer) = %p, want original pointer %p", got, input)
		}
	})

	t.Run("returns copied pointer for value payload", func(t *testing.T) {
		input := tmux.PaneOutputEvent{PaneID: "%2", Data: []byte("xyz")}
		got, ok := paneOutputEventFromPayload(input)
		if !ok {
			t.Fatal("paneOutputEventFromPayload(value) should be handled")
		}
		if got == nil {
			t.Fatal("paneOutputEventFromPayload(value) = nil, want non-nil")
		}
		if got.PaneID != input.PaneID || string(got.Data) != string(input.Data) {
			t.Fatalf("paneOutputEventFromPayload(value) = %+v, want %+v", got, input)
		}
		got.PaneID = "%changed"
		if input.PaneID != "%2" {
			t.Fatalf("value payload should be copied, input.PaneID = %q", input.PaneID)
		}
	})

	t.Run("returns nil for nil pointer payload", func(t *testing.T) {
		var input *tmux.PaneOutputEvent
		got, ok := paneOutputEventFromPayload(input)
		if !ok {
			t.Fatal("paneOutputEventFromPayload(nil pointer) should be handled")
		}
		if got != nil {
			t.Fatalf("paneOutputEventFromPayload(nil pointer) = %+v, want nil", got)
		}
	})

	t.Run("returns nil for unsupported payload", func(t *testing.T) {
		got, ok := paneOutputEventFromPayload(struct{}{})
		if ok {
			t.Fatal("paneOutputEventFromPayload(unsupported) should not be handled")
		}
		if got != nil {
			t.Fatalf("paneOutputEventFromPayload(unsupported) = %+v, want nil", got)
		}
	})
}

func TestEmitBackendEventPaneOutputSkipsEmptyMapPayloadWithoutWarning(t *testing.T) {
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
	app.emitBackendEvent("tmux:pane-output", map[string]any{
		"paneId": " %1 ",
		"data":   "",
	})

	select {
	case item := <-app.paneFeedCh:
		putFeedBuffer(item.poolPtr)
		t.Fatalf("unexpected pane output enqueue: %+v", item)
	default:
	}

	logOutput := buf.String()
	if strings.Contains(logOutput, "[EVENT] pane-output: unexpected payload type") {
		t.Fatalf("unexpected warning log for empty payload, output=%q", logOutput)
	}
	if !strings.Contains(logOutput, "skip empty pane-output payload") {
		t.Fatalf("missing debug skip log for empty payload, output=%q", logOutput)
	}
}

func TestEmitBackendEventPaneOutputLogsRawPayloadForUnknownTypes(t *testing.T) {
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
	if !strings.Contains(logOutput, "rawPayload=") {
		t.Fatalf("missing rawPayload field in log output=%q", logOutput)
	}
	if !strings.Contains(logOutput, "unexpected") {
		t.Fatalf("missing payload value in log output=%q", logOutput)
	}
}

func TestShouldEmitSnapshotForEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		want      bool
	}{
		{name: "session created", eventName: "tmux:session-created", want: true},
		{name: "session destroyed", eventName: "tmux:session-destroyed", want: true},
		{name: "pane created", eventName: "tmux:pane-created", want: true},
		{name: "layout changed", eventName: "tmux:layout-changed", want: true},
		{name: "pane focused", eventName: "tmux:pane-focused", want: true},
		{name: "pane renamed", eventName: "tmux:pane-renamed", want: true},
		{name: "session renamed", eventName: "tmux:session-renamed", want: true},
		{name: "window destroyed", eventName: "tmux:window-destroyed", want: true},
		{name: "window renamed", eventName: "tmux:window-renamed", want: true},
		{name: "window created (policy registered, no emitter yet)", eventName: "tmux:window-created", want: true},
		{name: "legacy alias removed", eventName: "window-renamed", want: false},
		{name: "non trigger", eventName: "tmux:pane-output", want: false},
		{name: "unknown", eventName: "noop", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldEmitSnapshotForEvent(tt.eventName); got != tt.want {
				t.Fatalf("shouldEmitSnapshotForEvent(%q) = %v, want %v", tt.eventName, got, tt.want)
			}
		})
	}
}

func TestShouldBypassSnapshotDebounceForEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		want      bool
	}{
		{name: "session created", eventName: "tmux:session-created", want: true},
		{name: "session destroyed", eventName: "tmux:session-destroyed", want: true},
		{name: "pane focused", eventName: "tmux:pane-focused", want: true},
		{name: "pane renamed", eventName: "tmux:pane-renamed", want: true},
		{name: "session renamed", eventName: "tmux:session-renamed", want: true},
		{name: "window destroyed", eventName: "tmux:window-destroyed", want: true},
		{name: "window renamed", eventName: "tmux:window-renamed", want: true},
		{name: "window created (policy registered, no emitter yet)", eventName: "tmux:window-created", want: true},
		{name: "layout changed", eventName: "tmux:layout-changed", want: false},
		{name: "pane created", eventName: "tmux:pane-created", want: false},
		{name: "unknown event", eventName: "tmux:unknown", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBypassSnapshotDebounceForEvent(tt.eventName); got != tt.want {
				t.Fatalf("shouldBypassSnapshotDebounceForEvent(%q) = %v, want %v", tt.eventName, got, tt.want)
			}
		})
	}
}

func TestRequestSnapshotNilGuards(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		t.Fatalf("unexpected runtime event emission for nil-guard path: %s", name)
	}

	t.Run("runtime context is nil", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()

		app.requestSnapshot(false)

		app.snapshotRequestMu.Lock()
		defer app.snapshotRequestMu.Unlock()
		if app.snapshotRequestTimer != nil {
			t.Fatal("snapshotRequestTimer should remain nil when runtime context is nil")
		}
		if app.snapshotRequestGeneration != 0 || app.snapshotRequestDispatched != 0 {
			t.Fatalf("snapshot request state mutated unexpectedly: generation=%d dispatched=%d",
				app.snapshotRequestGeneration, app.snapshotRequestDispatched)
		}
	})

	t.Run("sessions manager is nil", func(t *testing.T) {
		app := NewApp()
		app.setRuntimeContext(context.Background())
		app.sessions = nil

		app.requestSnapshot(false)

		app.snapshotRequestMu.Lock()
		defer app.snapshotRequestMu.Unlock()
		if app.snapshotRequestTimer != nil {
			t.Fatal("snapshotRequestTimer should remain nil when sessions manager is nil")
		}
		if app.snapshotRequestGeneration != 0 || app.snapshotRequestDispatched != 0 {
			t.Fatalf("snapshot request state mutated unexpectedly: generation=%d dispatched=%d",
				app.snapshotRequestGeneration, app.snapshotRequestDispatched)
		}
	})
}

func TestClearSnapshotRequestTimerStopsScheduledCallback(t *testing.T) {
	app := NewApp()

	triggered := make(chan struct{}, 1)
	app.snapshotRequestMu.Lock()
	app.snapshotRequestTimer = time.AfterFunc(20*time.Millisecond, func() {
		select {
		case triggered <- struct{}{}:
		default:
		}
	})
	app.snapshotRequestMu.Unlock()

	app.clearSnapshotRequestTimer()
	app.clearSnapshotRequestTimer()

	select {
	case <-triggered:
		t.Fatal("snapshot request timer callback should not fire after clearSnapshotRequestTimer")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestRequestSnapshotCoalescesBurst(t *testing.T) {
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

	for range 6 {
		app.requestSnapshot(false)
	}
	// 2 s timeout: coalesce window is 50 ms; allow headroom for CI scheduler jitter.
	waitForCondition(
		t,
		2*time.Second,
		func() bool {
			mu.Lock()
			defer mu.Unlock()
			return snapshotEvents >= 1
		},
		"coalesced snapshot emission",
	)

	mu.Lock()
	defer mu.Unlock()
	if snapshotEvents != 1 {
		t.Fatalf("snapshot event count = %d, want 1 for coalesced burst", snapshotEvents)
	}
}

func TestRequestSnapshotImmediateCancelsPendingDebounce(t *testing.T) {
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

	app.requestSnapshot(false)
	app.requestSnapshot(true)
	// 2 s timeout: coalesce window is 50 ms; allow headroom for CI scheduler jitter.
	waitForCondition(
		t,
		2*time.Second,
		func() bool {
			mu.Lock()
			defer mu.Unlock()
			return snapshotEvents >= 1
		},
		"immediate snapshot emission",
	)

	mu.Lock()
	defer mu.Unlock()
	if snapshotEvents != 1 {
		t.Fatalf("snapshot event count = %d, want 1 with immediate bypass", snapshotEvents)
	}
}

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
	// 2 s timeout: coalesce window is 50 ms; allow headroom for CI scheduler jitter.
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

func TestShouldSyncPaneStates(t *testing.T) {
	app := NewApp()

	if !app.shouldSyncPaneStates(1) {
		t.Fatal("first topology generation should require sync")
	}
	if app.shouldSyncPaneStates(1) {
		t.Fatal("same topology generation should not require sync")
	}
	if !app.shouldSyncPaneStates(2) {
		t.Fatal("new topology generation should require sync")
	}
}

func TestSyncPaneStatesIntegration(t *testing.T) {
	t.Run("nil paneStates is a no-op", func(t *testing.T) {
		app := NewApp()
		app.paneStates = nil
		// Must not panic.
		app.syncPaneStates([]tmux.SessionSnapshot{{Name: "test"}})
	})

	t.Run("EnsurePane is called for each snapshot pane", func(t *testing.T) {
		app := NewApp()
		// Replace with a small-capacity manager so Feed is observable.
		app.paneStates = panestate.NewManager(8 * 1024)

		snapshots := []tmux.SessionSnapshot{
			{
				Name: "s1",
				Windows: []tmux.WindowSnapshot{
					{
						Panes: []tmux.PaneSnapshot{
							{ID: "%1", Index: 0, Active: true, Width: 80, Height: 24},
						},
					},
				},
			},
		}
		app.syncPaneStates(snapshots)

		// EnsurePane registers the pane; Feed writes data which becomes observable via Snapshot.
		app.paneStates.Feed("%1", []byte("hello"))
		if got := app.paneStates.Snapshot("%1"); got == "" {
			t.Fatal("Snapshot(%%1) = empty after EnsurePane+Feed, want non-empty")
		}
	})

	t.Run("RetainPanes removes panes absent from the snapshot", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(8 * 1024)

		// Pre-populate a stale pane with data so Snapshot returns non-empty.
		app.paneStates.EnsurePane("%stale", 80, 24)
		app.paneStates.Feed("%stale", []byte("old data"))
		if app.paneStates.Snapshot("%stale") == "" {
			t.Fatal("setup: Snapshot(stale pane) is empty before syncPaneStates")
		}

		// syncPaneStates with only %alive — %stale should be removed.
		snapshots := []tmux.SessionSnapshot{
			{
				Name: "s1",
				Windows: []tmux.WindowSnapshot{
					{
						Panes: []tmux.PaneSnapshot{
							{ID: "%alive", Index: 0, Active: true, Width: 80, Height: 24},
						},
					},
				},
			},
		}
		app.syncPaneStates(snapshots)

		if got := app.paneStates.Snapshot("%stale"); got != "" {
			t.Fatalf("Snapshot(%%stale) = %q, want empty after RetainPanes removed stale pane", got)
		}
	})

	t.Run("SetActivePanes reflects active pane from snapshot", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(8 * 1024)

		snapshots := []tmux.SessionSnapshot{
			{
				Name: "s1",
				Windows: []tmux.WindowSnapshot{
					{
						Panes: []tmux.PaneSnapshot{
							{ID: "%10", Index: 0, Active: true, Width: 80, Height: 24},
							{ID: "%11", Index: 1, Active: false, Width: 80, Height: 24},
						},
					},
				},
			},
		}
		// Call twice to exercise SetActivePanes on subsequent topology changes.
		app.syncPaneStates(snapshots)

		// Toggle active pane to %11 on second call.
		snapshots[0].Windows[0].Panes[0].Active = false
		snapshots[0].Windows[0].Panes[1].Active = true
		app.syncPaneStates(snapshots)

		// Both panes should still be accessible after two calls.
		app.paneStates.Feed("%10", []byte("p10"))
		app.paneStates.Feed("%11", []byte("p11"))
		if app.paneStates.Snapshot("%10") == "" {
			t.Fatal("Snapshot(%%10) empty after second syncPaneStates call")
		}
		if app.paneStates.Snapshot("%11") == "" {
			t.Fatal("Snapshot(%%11) empty after second syncPaneStates call")
		}
	})
}

func TestEmitSnapshot(t *testing.T) {
	tests := []struct {
		name          string
		setupCtx      bool // whether to set a non-nil runtime context
		setupSessions bool // whether to initialize sessions manager
		createSession bool // whether to create a session for snapshot data
		wantEvent     string
		wantSkip      bool // expect no event emission
	}{
		{
			name:          "skips when runtime context is nil",
			setupCtx:      false,
			setupSessions: true,
			wantSkip:      true,
		},
		{
			name:          "skips when sessions manager is nil",
			setupCtx:      true,
			setupSessions: false,
			wantSkip:      true,
		},
		{
			name:          "emits full snapshot on first call",
			setupCtx:      true,
			setupSessions: true,
			createSession: true,
			wantEvent:     "tmux:snapshot",
		},
		{
			name:          "emits full snapshot even without sessions",
			setupCtx:      true,
			setupSessions: true,
			createSession: false,
			wantEvent:     "tmux:snapshot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origEmit := runtimeEventsEmitFn
			t.Cleanup(func() {
				runtimeEventsEmitFn = origEmit
			})

			var emittedEvents []string
			var capturedPayload any
			runtimeEventsEmitFn = func(_ context.Context, name string, payload ...any) {
				emittedEvents = append(emittedEvents, name)
				if name == "tmux:snapshot" && len(payload) > 0 {
					capturedPayload = payload[0]
				}
			}

			app := NewApp()
			if tt.setupCtx {
				app.setRuntimeContext(context.Background())
			}
			if tt.setupSessions {
				app.sessions = tmux.NewSessionManager()
				if tt.createSession {
					if _, _, err := app.sessions.CreateSession("verify-session", "0", 80, 24); err != nil {
						t.Fatalf("CreateSession() error = %v", err)
					}
				}
			}

			app.emitSnapshot()

			if tt.wantSkip {
				if len(emittedEvents) != 0 {
					t.Fatalf("expected no events, got %v", emittedEvents)
				}
				return
			}

			if len(emittedEvents) == 0 {
				t.Fatalf("expected event %q, got none", tt.wantEvent)
			}
			if emittedEvents[0] != tt.wantEvent {
				t.Fatalf("emitted event = %q, want %q", emittedEvents[0], tt.wantEvent)
			}

			// Verify payload content when snapshot contains session data
			if tt.name == "emits full snapshot on first call" && tt.createSession {
				snapshots, ok := capturedPayload.([]tmux.SessionSnapshot)
				if !ok || len(snapshots) == 0 {
					t.Fatalf("snapshot payload = %T %v, want []SessionSnapshot with at least 1 session", capturedPayload, capturedPayload)
				}
				if snapshots[0].Name != "verify-session" {
					t.Fatalf("snapshot[0].Name = %q, want %q", snapshots[0].Name, "verify-session")
				}
			}
		})
	}

	t.Run("second call with unchanged data emits no event", func(t *testing.T) {
		origEmit := runtimeEventsEmitFn
		t.Cleanup(func() {
			runtimeEventsEmitFn = origEmit
		})

		var emittedEvents []string
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			emittedEvents = append(emittedEvents, name)
		}

		app := NewApp()
		app.setRuntimeContext(context.Background())
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("stable", "0", 80, 24); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		// First call: populates cache, emits full snapshot.
		app.emitSnapshot()
		if len(emittedEvents) != 1 || emittedEvents[0] != "tmux:snapshot" {
			t.Fatalf("first call: events = %v, want [tmux:snapshot]", emittedEvents)
		}

		// Second call: same data, no delta.
		emittedEvents = nil
		app.emitSnapshot()
		if len(emittedEvents) != 0 {
			t.Fatalf("second call with unchanged data: events = %v, want none", emittedEvents)
		}
	})

	t.Run("emits delta after session rename", func(t *testing.T) {
		origEmit := runtimeEventsEmitFn
		t.Cleanup(func() {
			runtimeEventsEmitFn = origEmit
		})

		var emittedEvents []string
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			emittedEvents = append(emittedEvents, name)
		}

		app := NewApp()
		app.setRuntimeContext(context.Background())
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("before", "0", 80, 24); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		// Populate cache.
		app.emitSnapshot()
		emittedEvents = nil

		// Mutate session data to force a delta.
		if err := app.sessions.RenameSession("before", "after"); err != nil {
			t.Fatalf("RenameSession() error = %v", err)
		}

		app.emitSnapshot()
		if len(emittedEvents) == 0 {
			t.Fatal("expected delta event after session rename, got none")
		}
		if emittedEvents[0] != "tmux:snapshot-delta" {
			t.Fatalf("emitted event = %q, want %q", emittedEvents[0], "tmux:snapshot-delta")
		}
	})
}

func TestSnapshotEventPolicyConsistency(t *testing.T) {
	// Every event that bypasses debounce must also trigger a snapshot.
	for name, policy := range snapshotEventPolicies {
		if policy.bypassDebounce && !policy.trigger {
			t.Errorf("snapshotEventPolicies[%q]: bypassDebounce=true but trigger=false is inconsistent", name)
		}
	}
}

// TestAllRegisteredEventsHaveSnapshotPolicyTests ensures that every event registered
// in snapshotEventPolicies is covered by shouldEmitSnapshotForEvent and
// shouldBypassSnapshotDebounceForEvent, and that the functions return values
// consistent with the policy map.
//
// Role division:
//   - TestShouldEmitSnapshotForEvent / TestShouldBypassSnapshotDebounceForEvent:
//     human-readable table-driven tests that document each event's expected behavior.
//   - TestAllRegisteredEventsHaveSnapshotPolicyTests (this test):
//     exhaustive consistency guard that programmatically iterates snapshotEventPolicies
//     to catch newly added events missing from the table-driven tests above.
//   - TestSnapshotEventPolicyConsistency: structural invariant check (bypassDebounce
//     must not be true when trigger is false).
func TestAllRegisteredEventsHaveSnapshotPolicyTests(t *testing.T) {
	for name, policy := range snapshotEventPolicies {
		t.Run(name, func(t *testing.T) {
			gotTrigger := shouldEmitSnapshotForEvent(name)
			if gotTrigger != policy.trigger {
				t.Errorf("shouldEmitSnapshotForEvent(%q) = %v, want %v (from policy)", name, gotTrigger, policy.trigger)
			}
			gotBypass := shouldBypassSnapshotDebounceForEvent(name)
			if gotBypass != policy.bypassDebounce {
				t.Errorf("shouldBypassSnapshotDebounceForEvent(%q) = %v, want %v (from policy)", name, gotBypass, policy.bypassDebounce)
			}
		})
	}

	// Negative cases: events NOT in the policy map must return false for both.
	unregistered := []string{
		"tmux:pane-output",
		"tmux:unknown-event",
		"app:activate-window",
		"",
	}
	for _, name := range unregistered {
		t.Run("unregistered/"+name, func(t *testing.T) {
			if shouldEmitSnapshotForEvent(name) {
				t.Errorf("shouldEmitSnapshotForEvent(%q) = true for unregistered event", name)
			}
			if shouldBypassSnapshotDebounceForEvent(name) {
				t.Errorf("shouldBypassSnapshotDebounceForEvent(%q) = true for unregistered event", name)
			}
		})
	}
}

func TestLegacyMapPaneOutputTypeMismatchLog(t *testing.T) {
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

	tests := []struct {
		name        string
		dataField   any
		wantTypeLog bool // expect the type-mismatch debug log
	}{
		{
			name:        "int triggers type mismatch log",
			dataField:   42,
			wantTypeLog: true,
		},
		{
			name:        "struct triggers type mismatch log",
			dataField:   struct{ X int }{X: 1},
			wantTypeLog: true,
		},
		{
			name:        "nil does not trigger type mismatch log",
			dataField:   nil,
			wantTypeLog: false,
		},
		{
			name:        "string does not trigger type mismatch log",
			dataField:   "valid",
			wantTypeLog: false,
		},
		{
			name:        "byte slice does not trigger type mismatch log",
			dataField:   []byte("valid"),
			wantTypeLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			app := NewApp()
			app.setRuntimeContext(context.Background())
			t.Cleanup(func() {
				app.stopOutputBuffer("%1")
				app.detachAllOutputBuffers()
			})

			app.emitBackendEvent("tmux:pane-output", map[string]any{
				"paneId": "%1",
				"data":   tt.dataField,
			})

			logOutput := buf.String()
			hasTypeLog := strings.Contains(logOutput, "unsupported data field type")
			if hasTypeLog != tt.wantTypeLog {
				t.Fatalf("type mismatch log present=%v, want=%v; log=%q", hasTypeLog, tt.wantTypeLog, logOutput)
			}
		})
	}
}

func TestDetachOutputBuffers(t *testing.T) {
	app := NewApp()
	t.Cleanup(func() {
		app.detachAllOutputBuffers()
	})
	flusher := app.ensureOutputFlusher()
	flusher.Write("%1", []byte("keep"))
	flusher.Write("%2", []byte("drop"))

	removed := app.detachStaleOutputBuffers(map[string]struct{}{"%1": {}})
	if len(removed) != 1 || removed[0] != "%2" {
		t.Fatalf("detachStaleOutputBuffers() = %#v, want [%q]", removed, "%2")
	}

	all := app.detachAllOutputBuffers()
	if len(all) != 1 || all[0] != "%1" {
		t.Fatalf("detachAllOutputBuffers() = %#v, want [%q]", all, "%1")
	}

	app.outputMu.Lock()
	defer app.outputMu.Unlock()
	if app.outputFlusher != nil {
		t.Fatal("outputFlusher should be nil after detachAllOutputBuffers")
	}
}

// TestEnsureOutputFlusherFallbackToIPC_NilHub verifies that when wsHub is nil,
// flushed pane data is sent via Wails IPC (emitRuntimeEventWithContext) (C-1).
func TestEnsureOutputFlusherFallbackToIPC_NilHub(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	var mu sync.Mutex
	var ipcEvents []string
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		mu.Lock()
		ipcEvents = append(ipcEvents, name)
		mu.Unlock()
	}

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.wsHub = nil // explicit nil: force IPC path

	t.Cleanup(func() { app.detachAllOutputBuffers() })

	app.enqueuePaneOutput("%ipc-nil", []byte("hello ipc"))

	// Wait for the OutputFlushManager's flush tick (outputFlushInterval = 16ms).
	waitForCondition(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Contains(ipcEvents, "pane:data:%ipc-nil")
	}, "IPC event for pane:data:%ipc-nil when wsHub is nil")
}

// TestEnsureOutputFlusherFallbackToIPC_NoActiveConnection verifies that when
// wsHub is not nil but has no active WebSocket connection, IPC is used (C-1).
func TestEnsureOutputFlusherFallbackToIPC_NoActiveConnection(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	var mu sync.Mutex
	var ipcEvents []string
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		mu.Lock()
		ipcEvents = append(ipcEvents, name)
		mu.Unlock()
	}

	// Start a hub but do NOT connect any WebSocket client.
	hub := wsserver.NewHub(wsserver.HubOptions{Addr: "127.0.0.1:0"})
	ctx := t.Context()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("hub.Start() error: %v", err)
	}
	t.Cleanup(func() {
		if err := hub.Stop(); err != nil {
			t.Logf("hub.Stop() error: %v", err)
		}
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.wsHub = hub // hub is not nil, but HasActiveConnection() == false

	t.Cleanup(func() { app.detachAllOutputBuffers() })

	app.enqueuePaneOutput("%ipc-noconn", []byte("hello no conn"))

	waitForCondition(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Contains(ipcEvents, "pane:data:%ipc-noconn")
	}, "IPC event for pane:data:%ipc-noconn when hub has no active connection")
}

// TestEnsureOutputFlusherWebSocketPath verifies that when wsHub has an active
// WebSocket connection, data is sent via WebSocket (NOT via IPC) (C-1).
func TestEnsureOutputFlusherWebSocketPath(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	var mu sync.Mutex
	var ipcEvents []string
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		mu.Lock()
		ipcEvents = append(ipcEvents, name)
		mu.Unlock()
	}

	hub := wsserver.NewHub(wsserver.HubOptions{Addr: "127.0.0.1:0"})
	ctx := t.Context()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("hub.Start() error: %v", err)
	}
	t.Cleanup(func() {
		if err := hub.Stop(); err != nil {
			t.Logf("hub.Stop() error: %v", err)
		}
	})

	// Connect a WebSocket client and subscribe to the pane.
	u, parseErr := url.Parse(hub.URL())
	if parseErr != nil {
		t.Fatalf("url.Parse(%q) error: %v", hub.URL(), parseErr)
	}
	wsConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := wsConn.Close(); closeErr != nil {
			t.Logf("wsConn.Close(): %v", closeErr)
		}
	})

	// Subscribe the client to the test pane.
	subMsg, marshalErr := json.Marshal(map[string]any{"action": "subscribe", "paneIds": []string{"%ws-active"}})
	if marshalErr != nil {
		t.Fatalf("json.Marshal() error: %v", marshalErr)
	}
	if writeErr := wsConn.WriteMessage(websocket.TextMessage, subMsg); writeErr != nil {
		t.Fatalf("subscribe write error: %v", writeErr)
	}
	// Poll until the hub registers the subscription (I-23: replaces time.Sleep).
	waitForCondition(t, 2*time.Second, func() bool {
		return hub.HasActiveConnection()
	}, "hub to register active connection and process subscription")

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.wsHub = hub

	t.Cleanup(func() { app.detachAllOutputBuffers() })

	app.enqueuePaneOutput("%ws-active", []byte("ws data"))

	// Read from WebSocket expecting the binary frame.
	if err := wsConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	msgType, msg, readErr := wsConn.ReadMessage()
	if readErr != nil {
		t.Fatalf("ReadMessage() error: %v", readErr)
	}
	if msgType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want BinaryMessage", msgType)
	}
	if len(msg) == 0 {
		t.Fatal("received empty WebSocket message")
	}

	// I-24: Allow a short drain period before asserting IPC absence to reduce
	// race condition risk. The OutputFlushManager flushes at 16ms intervals;
	// 50ms gives ~3 flush cycles for any pending IPC events to arrive.
	time.Sleep(50 * time.Millisecond)

	// Verify no IPC event was emitted for this pane.
	mu.Lock()
	for _, name := range ipcEvents {
		if name == "pane:data:%ws-active" {
			mu.Unlock()
			t.Fatalf("IPC event emitted for pane:data:%%ws-active; expected WebSocket path only")
		}
	}
	mu.Unlock()
}

// TestGetWebSocketURL verifies the GetWebSocketURL public API (I-7).
func TestGetWebSocketURL(t *testing.T) {
	t.Run("returns empty string when wsHub is nil", func(t *testing.T) {
		app := NewApp()
		// wsHub is nil by default in NewApp().
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
