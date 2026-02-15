package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

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
