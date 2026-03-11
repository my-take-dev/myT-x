package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

// ------------------------------------------------------------
// Validation tests
// ------------------------------------------------------------

func TestStartSchedulerValidation(t *testing.T) {
	tests := []struct {
		name            string
		title           string
		paneID          string
		message         string
		intervalMinutes int
		maxCount        int
		wantErr         string
	}{
		{
			name:            "empty title",
			title:           "",
			paneID:          "%1",
			message:         "hello",
			intervalMinutes: 1,
			maxCount:        1,
			wantErr:         "title is required",
		},
		{
			name:            "whitespace only title",
			title:           "   ",
			paneID:          "%1",
			message:         "hello",
			intervalMinutes: 1,
			maxCount:        1,
			wantErr:         "title is required",
		},
		{
			name:            "empty pane id",
			title:           "test",
			paneID:          "",
			message:         "hello",
			intervalMinutes: 1,
			maxCount:        1,
			wantErr:         "pane id is required",
		},
		{
			name:            "empty message",
			title:           "test",
			paneID:          "%1",
			message:         "",
			intervalMinutes: 1,
			maxCount:        1,
			wantErr:         "message is required",
		},
		{
			name:            "interval zero",
			title:           "test",
			paneID:          "%1",
			message:         "hello",
			intervalMinutes: 0,
			maxCount:        1,
			wantErr:         "interval must be at least 1 minute",
		},
		{
			name:            "interval negative",
			title:           "test",
			paneID:          "%1",
			message:         "hello",
			intervalMinutes: -1,
			maxCount:        1,
			wantErr:         "interval must be at least 1 minute",
		},
		{
			name:            "max count negative",
			title:           "test",
			paneID:          "%1",
			message:         "hello",
			intervalMinutes: 1,
			maxCount:        -1,
			wantErr:         "send count must be 0 for infinite or at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()

			_, err := app.StartScheduler(tt.title, tt.paneID, tt.message, tt.intervalMinutes, tt.maxCount)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Errorf("error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestStartSchedulerPaneNotFound(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, err := app.StartScheduler("test", "%999", "hello", 1, 1)
	if err == nil {
		t.Fatal("expected error for non-existent pane")
	}
}

// ------------------------------------------------------------
// StopScheduler tests
// ------------------------------------------------------------

func TestStopSchedulerNotFound(t *testing.T) {
	app := NewApp()
	err := app.StopScheduler("non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent scheduler")
	}
}

func TestStopSchedulerEmptyID(t *testing.T) {
	app := NewApp()
	err := app.StopScheduler("")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestStopSchedulerMarksEntryStopped(t *testing.T) {
	app := NewApp()

	// Manually add an entry.
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	entry := &schedulerEntry{
		ID:       "test-id",
		Title:    "test",
		PaneID:   "%1",
		Running:  true,
		RunToken: 1,
		cancel:   cancel,
	}
	app.schedulerMu.Lock()
	app.schedulerEntries["test-id"] = entry
	app.schedulerMu.Unlock()

	err := app.StopScheduler("test-id")
	if err != nil {
		t.Fatalf("StopScheduler() error = %v", err)
	}

	app.schedulerMu.Lock()
	stoppedEntry, exists := app.schedulerEntries["test-id"]
	app.schedulerMu.Unlock()

	if !exists {
		t.Fatal("entry should remain after stop")
	}
	if stoppedEntry.Running {
		t.Fatal("entry should be marked stopped")
	}
}

func TestResumeSchedulerRestartsStoppedEntry(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.schedulerMu.Lock()
	app.schedulerEntries["resume-id"] = &schedulerEntry{
		ID:              "resume-id",
		Title:           "resume",
		PaneID:          pane.IDString(),
		Message:         "hello",
		IntervalMinutes: 1,
		MaxCount:        2,
		CurrentCount:    5,
		Running:         false,
		RunToken:        2,
		cancel:          cancel,
	}
	app.schedulerMu.Unlock()

	err = app.ResumeScheduler("resume-id")
	if err != nil {
		t.Fatalf("ResumeScheduler() error = %v", err)
	}

	app.schedulerMu.Lock()
	entry := app.schedulerEntries["resume-id"]
	app.schedulerMu.Unlock()

	if !entry.Running {
		t.Fatal("entry should be running after resume")
	}
	if entry.CurrentCount != 0 {
		t.Fatalf("CurrentCount = %d, want 0", entry.CurrentCount)
	}
	if entry.RunToken != 3 {
		t.Fatalf("RunToken = %d, want 3", entry.RunToken)
	}
}

func TestDeleteSchedulerRemovesEntry(t *testing.T) {
	app := NewApp()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.schedulerMu.Lock()
	app.schedulerEntries["delete-id"] = &schedulerEntry{
		ID:      "delete-id",
		Title:   "delete",
		Running: false,
		cancel:  cancel,
	}
	app.schedulerMu.Unlock()

	err := app.DeleteScheduler("delete-id")
	if err != nil {
		t.Fatalf("DeleteScheduler() error = %v", err)
	}

	app.schedulerMu.Lock()
	_, exists := app.schedulerEntries["delete-id"]
	app.schedulerMu.Unlock()
	if exists {
		t.Fatal("entry should be removed after delete")
	}
}

// ------------------------------------------------------------
// StopAllSchedulers tests
// ------------------------------------------------------------

func TestStopAllSchedulers(t *testing.T) {
	app := NewApp()

	// Add multiple entries.
	app.schedulerMu.Lock()
	for _, id := range []string{"a", "b", "c"} {
		_, cancel := context.WithCancel(context.Background())
		app.schedulerEntries[id] = &schedulerEntry{
			ID:       id,
			Title:    id,
			PaneID:   "%1",
			Running:  true,
			RunToken: 1,
			cancel:   cancel,
		}
	}
	app.schedulerMu.Unlock()

	err := app.StopAllSchedulers()
	if err != nil {
		t.Fatalf("StopAllSchedulers() error = %v", err)
	}

	app.schedulerMu.Lock()
	if len(app.schedulerEntries) != 3 {
		app.schedulerMu.Unlock()
		t.Fatalf("expected 3 entries, got %d", len(app.schedulerEntries))
	}
	for id, entry := range app.schedulerEntries {
		if entry.Running {
			app.schedulerMu.Unlock()
			t.Fatalf("entry %s should be stopped", id)
		}
	}
	app.schedulerMu.Unlock()
}

func TestStopAllSchedulersEmpty(t *testing.T) {
	app := NewApp()
	err := app.StopAllSchedulers()
	if err != nil {
		t.Fatalf("StopAllSchedulers() error = %v", err)
	}
}

// ------------------------------------------------------------
// GetSchedulerStatuses tests
// ------------------------------------------------------------

func TestGetSchedulerStatusesEmpty(t *testing.T) {
	app := NewApp()
	statuses := app.GetSchedulerStatuses()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestGetSchedulerStatusesCopiesData(t *testing.T) {
	app := NewApp()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.schedulerMu.Lock()
	app.schedulerEntries["x"] = &schedulerEntry{
		ID:              "x",
		Title:           "My Scheduler",
		PaneID:          "%5",
		Message:         "hello world",
		IntervalMinutes: 10,
		MaxCount:        50,
		CurrentCount:    3,
		Running:         false,
		cancel:          cancel,
	}
	app.schedulerMu.Unlock()

	statuses := app.GetSchedulerStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	s := statuses[0]
	if s.ID != "x" {
		t.Errorf("ID = %q, want %q", s.ID, "x")
	}
	if s.Title != "My Scheduler" {
		t.Errorf("Title = %q, want %q", s.Title, "My Scheduler")
	}
	if s.PaneID != "%5" {
		t.Errorf("PaneID = %q, want %q", s.PaneID, "%5")
	}
	if s.Message != "hello world" {
		t.Errorf("Message = %q, want %q", s.Message, "hello world")
	}
	if s.IntervalMinutes != 10 {
		t.Errorf("IntervalMinutes = %d, want %d", s.IntervalMinutes, 10)
	}
	if s.MaxCount != 50 {
		t.Errorf("MaxCount = %d, want %d", s.MaxCount, 50)
	}
	if s.CurrentCount != 3 {
		t.Errorf("CurrentCount = %d, want %d", s.CurrentCount, 3)
	}
	if s.Running {
		t.Error("Running = true, want false")
	}
}

func TestGetSchedulerStatusesSortedByID(t *testing.T) {
	app := NewApp()

	app.schedulerMu.Lock()
	app.schedulerEntries["b"] = &schedulerEntry{ID: "b", Title: "B", Running: true, cancel: func() {}}
	app.schedulerEntries["a"] = &schedulerEntry{ID: "a", Title: "A", Running: true, cancel: func() {}}
	app.schedulerEntries["c"] = &schedulerEntry{ID: "c", Title: "C", Running: true, cancel: func() {}}
	app.schedulerMu.Unlock()

	statuses := app.GetSchedulerStatuses()
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	if statuses[0].ID != "a" || statuses[1].ID != "b" || statuses[2].ID != "c" {
		t.Fatalf("unexpected status order: %q, %q, %q", statuses[0].ID, statuses[1].ID, statuses[2].ID)
	}
}

// ------------------------------------------------------------
// isPaneAlive tests
// ------------------------------------------------------------

func TestIsPaneAlive(t *testing.T) {
	sessions := tmux.NewSessionManager()
	_, pane, err := sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	paneID := pane.IDString()

	t.Run("existing pane returns true", func(t *testing.T) {
		if !isPaneAlive(sessions, paneID) {
			t.Errorf("isPaneAlive(%q) = false, want true", paneID)
		}
	})

	t.Run("non-existent pane returns false", func(t *testing.T) {
		if isPaneAlive(sessions, "%9999") {
			t.Error("isPaneAlive(%9999) = true, want false")
		}
	})
}

// ------------------------------------------------------------
// schedulerSendMessage tests
// ------------------------------------------------------------

func TestSchedulerSendMessage(t *testing.T) {
	origFn := executeRouterRequestFn
	t.Cleanup(func() { executeRouterRequestFn = origFn })

	t.Run("success", func(t *testing.T) {
		var captured ipc.TmuxRequest
		executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			captured = req
			return ipc.TmuxResponse{ExitCode: 0}
		}

		router := tmux.NewCommandRouter(tmux.NewSessionManager(), nil, tmux.RouterOptions{})
		err := schedulerSendMessage(router, "%1", "hello world")
		if err != nil {
			t.Fatalf("schedulerSendMessage() error = %v", err)
		}
		if captured.Command != "send-keys" {
			t.Errorf("Command = %q, want %q", captured.Command, "send-keys")
		}
		if captured.Flags["-t"] != "%1" {
			t.Errorf("Flags[-t] = %v, want %%1", captured.Flags["-t"])
		}
		if captured.Flags["-N"] != true {
			t.Errorf("Flags[-N] = %v, want true", captured.Flags["-N"])
		}
		if len(captured.Args) != 2 || captured.Args[0] != "hello world" || captured.Args[1] != "Enter" {
			t.Errorf("Args = %v, want [hello world Enter]", captured.Args)
		}
	})

	t.Run("failure returns error", func(t *testing.T) {
		executeRouterRequestFn = func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "pane not found"}
		}

		router := tmux.NewCommandRouter(tmux.NewSessionManager(), nil, tmux.RouterOptions{})
		err := schedulerSendMessage(router, "%1", "hello")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ------------------------------------------------------------
// removeSchedulerEntry tests
// ------------------------------------------------------------

func TestRemoveSchedulerEntry(t *testing.T) {
	app := NewApp()
	_, cancel := context.WithCancel(context.Background())

	app.schedulerMu.Lock()
	app.schedulerEntries["x"] = &schedulerEntry{
		ID:     "x",
		cancel: cancel,
	}
	app.schedulerMu.Unlock()

	app.removeSchedulerEntry("x")

	app.schedulerMu.Lock()
	_, exists := app.schedulerEntries["x"]
	app.schedulerMu.Unlock()

	if exists {
		t.Fatal("entry should have been removed")
	}
}

func TestRemoveSchedulerEntryNonExistent(t *testing.T) {
	app := NewApp()
	// Should not panic.
	app.removeSchedulerEntry("non-existent")
}

// ------------------------------------------------------------
// runSchedulerLoop context cancellation test
// ------------------------------------------------------------

func TestRunSchedulerLoopCancellation(t *testing.T) {
	app := NewApp()

	ctx, cancel := context.WithCancel(context.Background())
	entry := &schedulerEntry{
		ID:              "loop-test",
		Title:           "loop-test",
		PaneID:          "%1",
		Message:         "hello",
		IntervalMinutes: 60, // Long interval — we cancel immediately.
		MaxCount:        1,
		CurrentCount:    0,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}

	app.schedulerMu.Lock()
	app.schedulerEntries["loop-test"] = entry
	app.schedulerMu.Unlock()

	// runSchedulerLoop is called directly (not via RunWithPanicRecovery)
	// to test the loop body in isolation.
	var wg sync.WaitGroup
	wg.Go(func() {
		app.runSchedulerLoop(ctx, "loop-test", 1)
	})

	// Cancel immediately — the loop should exit.
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(5 * time.Second):
		t.Fatal("runSchedulerLoop did not exit after context cancellation")
	}
}

func TestRunSchedulerLoopPaneGoneEmitsStoppedEvent(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()

	var stopped map[string]string
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "scheduler:stopped" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]string)
		if !ok {
			t.Fatalf("unexpected payload type: %T", data[0])
		}
		stopped = payload
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.schedulerMu.Lock()
	app.schedulerEntries["gone"] = &schedulerEntry{
		ID:              "gone",
		Title:           "Gone",
		PaneID:          "%9",
		Message:         "hello",
		IntervalMinutes: 0,
		MaxCount:        1,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}
	app.schedulerMu.Unlock()

	app.runSchedulerLoop(ctx, "gone", 1)

	if stopped["title"] != "Gone" {
		t.Fatalf("stopped title = %q, want %q", stopped["title"], "Gone")
	}
	if stopped["reason"] != "target pane is no longer available" {
		t.Fatalf("stopped reason = %q", stopped["reason"])
	}
	app.schedulerMu.Lock()
	entry, exists := app.schedulerEntries["gone"]
	app.schedulerMu.Unlock()
	if !exists {
		t.Fatal("entry should remain after pane disappearance")
	}
	if entry.Running {
		t.Fatal("entry should be marked stopped after pane disappearance")
	}
}

func TestRunSchedulerLoopSendFailureEmitsStoppedEvent(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	origExec := executeRouterRequestFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
		executeRouterRequestFn = origExec
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	var stopped map[string]string
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "scheduler:stopped" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]string)
		if !ok {
			t.Fatalf("unexpected payload type: %T", data[0])
		}
		stopped = payload
	}
	executeRouterRequestFn = func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
		return ipc.TmuxResponse{ExitCode: 1, Stderr: "write failed"}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.schedulerMu.Lock()
	app.schedulerEntries["send-fail"] = &schedulerEntry{
		ID:              "send-fail",
		Title:           "Send Fail",
		PaneID:          pane.IDString(),
		Message:         "hello",
		IntervalMinutes: 0,
		MaxCount:        1,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}
	app.schedulerMu.Unlock()

	app.runSchedulerLoop(ctx, "send-fail", 1)

	if stopped["title"] != "Send Fail" {
		t.Fatalf("stopped title = %q, want %q", stopped["title"], "Send Fail")
	}
	if !strings.Contains(stopped["reason"], "message delivery failed") {
		t.Fatalf("stopped reason = %q", stopped["reason"])
	}
}

func TestRunSchedulerLoopMaxCountReached(t *testing.T) {
	origExec := executeRouterRequestFn
	t.Cleanup(func() { executeRouterRequestFn = origExec })

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	executeRouterRequestFn = func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
		return ipc.TmuxResponse{ExitCode: 0}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.schedulerMu.Lock()
	app.schedulerEntries["max-count"] = &schedulerEntry{
		ID:              "max-count",
		Title:           "Max Count",
		PaneID:          pane.IDString(),
		Message:         "hello",
		IntervalMinutes: 0,
		MaxCount:        1,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}
	app.schedulerMu.Unlock()

	app.runSchedulerLoop(ctx, "max-count", 1)

	app.schedulerMu.Lock()
	entry, exists := app.schedulerEntries["max-count"]
	app.schedulerMu.Unlock()
	if !exists {
		t.Fatal("entry should remain after reaching max count")
	}
	if entry.Running {
		t.Fatal("entry should be marked stopped after reaching max count")
	}
}

func TestRunSchedulerLoopInfiniteCountKeepsEntry(t *testing.T) {
	origExec := executeRouterRequestFn
	t.Cleanup(func() { executeRouterRequestFn = origExec })

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executeRouterRequestFn = func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
		app.schedulerMu.Lock()
		if entry, ok := app.schedulerEntries["infinite"]; ok {
			entry.IntervalMinutes = 1
		}
		app.schedulerMu.Unlock()
		cancel()
		return ipc.TmuxResponse{ExitCode: 0}
	}

	app.schedulerMu.Lock()
	app.schedulerEntries["infinite"] = &schedulerEntry{
		ID:              "infinite",
		Title:           "Infinite",
		PaneID:          pane.IDString(),
		Message:         "hello",
		IntervalMinutes: 0,
		MaxCount:        schedulerInfiniteCount,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}
	app.schedulerMu.Unlock()

	app.runSchedulerLoop(ctx, "infinite", 1)

	app.schedulerMu.Lock()
	entry, exists := app.schedulerEntries["infinite"]
	app.schedulerMu.Unlock()
	if !exists {
		t.Fatal("entry should remain for infinite scheduler")
	}
	if entry.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", entry.CurrentCount)
	}
}

// ------------------------------------------------------------
// Template persistence tests
// ------------------------------------------------------------

// setupTemplateTestApp creates an App with a session whose RootPath is a temp dir.
func setupTemplateTestApp(t *testing.T) (*App, string) {
	t.Helper()
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, _, err := app.sessions.CreateSession("test-session", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tmpDir := t.TempDir()
	if err := app.sessions.SetRootPath("test-session", tmpDir); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}
	return app, tmpDir
}

func TestSaveSchedulerTemplateValidation(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    SchedulerTemplate
		wantErr string
	}{
		{
			name:    "empty title",
			tmpl:    SchedulerTemplate{Title: "", Message: "hello", IntervalMinutes: 1, MaxCount: 1},
			wantErr: "title is required",
		},
		{
			name:    "whitespace-only title",
			tmpl:    SchedulerTemplate{Title: "   ", Message: "hello", IntervalMinutes: 1, MaxCount: 1},
			wantErr: "title is required",
		},
		{
			name:    "empty message",
			tmpl:    SchedulerTemplate{Title: "test", Message: "", IntervalMinutes: 1, MaxCount: 1},
			wantErr: "message is required",
		},
		{
			name:    "interval zero",
			tmpl:    SchedulerTemplate{Title: "test", Message: "hello", IntervalMinutes: 0, MaxCount: 1},
			wantErr: "interval must be at least 1 minute",
		},
		{
			name:    "max count negative",
			tmpl:    SchedulerTemplate{Title: "test", Message: "hello", IntervalMinutes: 1, MaxCount: -1},
			wantErr: "send count must be 0 for infinite or at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := setupTemplateTestApp(t)
			err := app.SaveSchedulerTemplate("test-session", tt.tmpl)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Errorf("error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestSaveAndLoadSchedulerTemplate(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	// Save a template.
	tmpl := SchedulerTemplate{
		Title:           "Deploy Check",
		Message:         "check deploy status",
		IntervalMinutes: 5,
		MaxCount:        10,
	}
	err := app.SaveSchedulerTemplate("test-session", tmpl)
	if err != nil {
		t.Fatalf("SaveSchedulerTemplate() error = %v", err)
	}

	// Load templates.
	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].Title != "Deploy Check" {
		t.Errorf("Title = %q, want %q", loaded[0].Title, "Deploy Check")
	}
	if loaded[0].Message != "check deploy status" {
		t.Errorf("Message = %q, want %q", loaded[0].Message, "check deploy status")
	}
	if loaded[0].IntervalMinutes != 5 {
		t.Errorf("IntervalMinutes = %d, want %d", loaded[0].IntervalMinutes, 5)
	}
	if loaded[0].MaxCount != 10 {
		t.Errorf("MaxCount = %d, want %d", loaded[0].MaxCount, 10)
	}
}

func TestSaveSchedulerTemplateOverwrite(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	// Save initial template.
	err := app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
		Title:           "Check",
		Message:         "original",
		IntervalMinutes: 1,
		MaxCount:        1,
	})
	if err != nil {
		t.Fatalf("first save error = %v", err)
	}

	// Save with same title but different content.
	err = app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
		Title:           "Check",
		Message:         "updated",
		IntervalMinutes: 10,
		MaxCount:        50,
	})
	if err != nil {
		t.Fatalf("second save error = %v", err)
	}

	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template (overwrite), got %d", len(loaded))
	}
	if loaded[0].Message != "updated" {
		t.Errorf("Message = %q, want %q", loaded[0].Message, "updated")
	}
	if loaded[0].IntervalMinutes != 10 {
		t.Errorf("IntervalMinutes = %d, want %d", loaded[0].IntervalMinutes, 10)
	}
}

func TestSaveSchedulerTemplatesSortedByTitle(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	for _, title := range []string{"Zebra", "Alpha", "Middle"} {
		err := app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
			Title:           title,
			Message:         "msg",
			IntervalMinutes: 1,
			MaxCount:        1,
		})
		if err != nil {
			t.Fatalf("save %q error = %v", title, err)
		}
	}

	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(loaded))
	}
	if loaded[0].Title != "Alpha" || loaded[1].Title != "Middle" || loaded[2].Title != "Zebra" {
		t.Errorf("templates not sorted: %v, %v, %v", loaded[0].Title, loaded[1].Title, loaded[2].Title)
	}
}

func TestDeleteSchedulerTemplate(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	// Save two templates.
	for _, title := range []string{"Keep", "Remove"} {
		err := app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
			Title:           title,
			Message:         "msg",
			IntervalMinutes: 1,
			MaxCount:        1,
		})
		if err != nil {
			t.Fatalf("save %q error = %v", title, err)
		}
	}

	// Delete one.
	err := app.DeleteSchedulerTemplate("test-session", "Remove")
	if err != nil {
		t.Fatalf("DeleteSchedulerTemplate() error = %v", err)
	}

	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].Title != "Keep" {
		t.Errorf("Title = %q, want %q", loaded[0].Title, "Keep")
	}
}

func TestDeleteSchedulerTemplateEmptyTitle(t *testing.T) {
	app, _ := setupTemplateTestApp(t)
	err := app.DeleteSchedulerTemplate("test-session", "")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestDeleteSchedulerTemplateNonExistent(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	// Delete from empty should succeed (no-op).
	err := app.DeleteSchedulerTemplate("test-session", "ghost")
	if err != nil {
		t.Fatalf("DeleteSchedulerTemplate() error = %v", err)
	}
}

func TestLoadSchedulerTemplatesNoFile(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	// No file saved yet — should return empty.
	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 templates, got %d", len(loaded))
	}
}

func TestLoadSchedulerTemplatesMalformedJSON(t *testing.T) {
	app, tmpDir := setupTemplateTestApp(t)

	// Write a malformed file.
	dir := filepath.Join(tmpDir, ".myT-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scheduler-templates.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v (expected nil for malformed)", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 templates for malformed JSON, got %d", len(loaded))
	}
}

func TestSaveSchedulerTemplateMalformedJSONReturnsError(t *testing.T) {
	app, tmpDir := setupTemplateTestApp(t)

	dir := filepath.Join(tmpDir, ".myT-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scheduler-templates.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
		Title:           "Check",
		Message:         "hello",
		IntervalMinutes: 1,
		MaxCount:        1,
	})
	if err == nil || !strings.Contains(err.Error(), "parse templates") {
		t.Fatalf("SaveSchedulerTemplate() error = %v, want parse templates error", err)
	}
}

func TestDeleteSchedulerTemplateMalformedJSONReturnsError(t *testing.T) {
	app, tmpDir := setupTemplateTestApp(t)

	dir := filepath.Join(tmpDir, ".myT-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scheduler-templates.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := app.DeleteSchedulerTemplate("test-session", "Check")
	if err == nil || !strings.Contains(err.Error(), "parse templates") {
		t.Fatalf("DeleteSchedulerTemplate() error = %v, want parse templates error", err)
	}
}

func TestResolveSchedulerTemplatePathSessionNotFound(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, err := app.resolveSchedulerTemplatePath("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestResolveSchedulerTemplatePathNoRootPath(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, _, err := app.sessions.CreateSession("no-root", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	// No root path set.
	_, err = app.resolveSchedulerTemplatePath("no-root")
	if err == nil {
		t.Fatal("expected error for session with no root path")
	}
}

func TestSchedulerTemplateFileFormat(t *testing.T) {
	app, tmpDir := setupTemplateTestApp(t)

	err := app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
		Title:           "Test",
		Message:         "hello\nworld",
		IntervalMinutes: 3,
		MaxCount:        99,
	})
	if err != nil {
		t.Fatalf("SaveSchedulerTemplate() error = %v", err)
	}

	// Read raw file and verify JSON structure.
	data, err := os.ReadFile(filepath.Join(tmpDir, ".myT-x", "scheduler-templates.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 entry in JSON, got %d", len(raw))
	}
	if raw[0]["title"] != "Test" {
		t.Errorf("title = %v, want %q", raw[0]["title"], "Test")
	}
}

func TestSaveSchedulerTemplateTitleTrimmed(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	err := app.SaveSchedulerTemplate("test-session", SchedulerTemplate{
		Title:           "  Padded  ",
		Message:         "msg",
		IntervalMinutes: 1,
		MaxCount:        1,
	})
	if err != nil {
		t.Fatalf("SaveSchedulerTemplate() error = %v", err)
	}

	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].Title != "Padded" {
		t.Errorf("Title = %q, want %q (trimmed)", loaded[0].Title, "Padded")
	}
}

func TestSaveSchedulerTemplateSessionNameTrimmed(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	err := app.SaveSchedulerTemplate("  test-session  ", SchedulerTemplate{
		Title:           "Trimmed Session",
		Message:         "msg",
		IntervalMinutes: 1,
		MaxCount:        0,
	})
	if err != nil {
		t.Fatalf("SaveSchedulerTemplate() error = %v", err)
	}

	loaded, err := app.LoadSchedulerTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadSchedulerTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].MaxCount != 0 {
		t.Fatalf("MaxCount = %d, want 0", loaded[0].MaxCount)
	}
}
