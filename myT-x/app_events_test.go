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
)

// NOTE: This file overrides package-level runtimeEventsEmitFn.
// Do not use t.Parallel() here.

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
	runtimeEventsEmitFn = func(context.Context, string, ...interface{}) {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
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
	runtimeEventsEmitFn = func(context.Context, string, ...interface{}) {}

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
	runtimeEventsEmitFn = func(context.Context, string, ...interface{}) {}

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
	runtimeEventsEmitFn = func(context.Context, string, ...interface{}) {}

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
	if strings.Contains(logOutput, "[DEBUG-EVENT] pane-output: unexpected payload type") {
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
	runtimeEventsEmitFn = func(context.Context, string, ...interface{}) {}

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
	if !strings.Contains(logOutput, "[DEBUG-EVENT] pane-output: unexpected payload type") {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		if name != "tmux:snapshot" && name != "tmux:snapshot-delta" {
			return
		}
		mu.Lock()
		snapshotEvents++
		mu.Unlock()
	}

	for i := 0; i < 6; i++ {
		app.requestSnapshot(false)
	}
	waitForCondition(
		t,
		snapshotCoalesceWindow+300*time.Millisecond,
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		if name != "tmux:snapshot" && name != "tmux:snapshot-delta" {
			return
		}
		mu.Lock()
		snapshotEvents++
		mu.Unlock()
	}

	app.requestSnapshot(false)
	app.requestSnapshot(true)
	waitForCondition(
		t,
		snapshotCoalesceWindow+300*time.Millisecond,
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		if name != "tmux:snapshot" && name != "tmux:snapshot-delta" {
			return
		}
		mu.Lock()
		snapshotEvents++
		mu.Unlock()
	}

	for i := 0; i < 4; i++ {
		app.emitBackendEvent("tmux:layout-changed", map[string]any{"sessionName": "alpha"})
	}
	waitForCondition(
		t,
		snapshotCoalesceWindow+300*time.Millisecond,
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
