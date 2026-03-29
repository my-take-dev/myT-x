package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/workerutil"
)

// ------------------------------------------------------------
// Test helpers
// ------------------------------------------------------------

// testDeps returns a Deps with minimal stubs for unit testing.
// Callers override individual fields as needed.
func testDeps() Deps {
	return Deps{
		IsShuttingDown: func() bool { return false },
		CheckPaneAlive: func(paneID string) error {
			return fmt.Errorf("pane %s does not exist", paneID)
		},
		SendMessage: func(paneID, message string) error {
			return nil
		},
		ResolveSessionRootPath: func(sessionName string) (string, error) {
			return "", fmt.Errorf("session %s not found", sessionName)
		},
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
		LaunchWorker: func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
			go fn(ctx)
		},
		BaseRecoveryOptions: func() workerutil.RecoveryOptions {
			return workerutil.RecoveryOptions{MaxRetries: 1}
		},
	}
}

// testDepsWithPanes returns a Deps where specified pane IDs are alive.
func testDepsWithPanes(alivePanes ...string) Deps {
	alive := map[string]bool{}
	for _, p := range alivePanes {
		alive[p] = true
	}
	d := testDeps()
	d.CheckPaneAlive = func(paneID string) error {
		if alive[paneID] {
			return nil
		}
		return fmt.Errorf("pane %s does not exist", paneID)
	}
	return d
}

// setupTemplateTestService creates a Service with ResolveSessionRootPath pointing to a temp dir.
func setupTemplateTestService(t *testing.T) (*Service, string) {
	t.Helper()
	tmpDir := t.TempDir()
	d := testDeps()
	d.ResolveSessionRootPath = func(sessionName string) (string, error) {
		if sessionName == "test-session" {
			return tmpDir, nil
		}
		return "", fmt.Errorf("session %s not found", sessionName)
	}
	return NewService(d), tmpDir
}

// ------------------------------------------------------------
// Validation tests
// ------------------------------------------------------------

func TestStartValidation(t *testing.T) {
	tests := []struct {
		name            string
		title           string
		paneID          string
		message         string
		intervalSeconds int
		maxCount        int
		wantErr         string
	}{
		{
			name: "empty title", title: "", paneID: "%1", message: "hello",
			intervalSeconds: 10, maxCount: 1, wantErr: "title is required",
		},
		{
			name: "whitespace only title", title: "   ", paneID: "%1", message: "hello",
			intervalSeconds: 10, maxCount: 1, wantErr: "title is required",
		},
		{
			name: "empty pane id", title: "test", paneID: "", message: "hello",
			intervalSeconds: 10, maxCount: 1, wantErr: "pane id is required",
		},
		{
			name: "interval zero", title: "test", paneID: "%1", message: "hello",
			intervalSeconds: 0, maxCount: 1, wantErr: "interval must be at least 10 seconds",
		},
		{
			name: "interval negative", title: "test", paneID: "%1", message: "hello",
			intervalSeconds: -1, maxCount: 1, wantErr: "interval must be at least 10 seconds",
		},
		{
			name: "interval too short", title: "test", paneID: "%1", message: "hello",
			intervalSeconds: 9, maxCount: 1, wantErr: "interval must be at least 10 seconds",
		},
		{
			name: "max count negative", title: "test", paneID: "%1", message: "hello",
			intervalSeconds: 10, maxCount: -1, wantErr: "send count must be 0 for infinite or at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps())
			_, err := svc.Start(tt.title, tt.paneID, tt.message, tt.intervalSeconds, tt.maxCount)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Errorf("error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestStartPaneNotFound(t *testing.T) {
	svc := NewService(testDeps())
	_, err := svc.Start("test", "%999", "hello", 10, 1)
	if err == nil {
		t.Fatal("expected error for non-existent pane")
	}
}

func TestStartEmptyMessageAllowed(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("auto-enter", "%1", "", 10, 0)
	if err != nil {
		t.Fatalf("Start() with empty message should succeed, got: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty scheduler id")
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Message != "" {
		t.Fatalf("expected empty message, got %q", statuses[0].Message)
	}
}

// ------------------------------------------------------------
// Stop tests
// ------------------------------------------------------------

func TestStopNotFound(t *testing.T) {
	svc := NewService(testDeps())
	err := svc.Stop("non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent scheduler")
	}
}

func TestStopEmptyID(t *testing.T) {
	svc := NewService(testDeps())
	err := svc.Stop("")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestStopMarksEntryStopped(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("test", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	err = svc.Stop(id)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Running {
		t.Fatal("entry should be marked stopped")
	}
}

// ------------------------------------------------------------
// Resume tests
// ------------------------------------------------------------

func TestResumeRestartsStoppedEntry(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("resume", "%1", "hello", 60, 2)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := svc.Stop(id); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if err := svc.Resume(id); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Running {
		t.Fatal("entry should be running after resume")
	}
	if statuses[0].CurrentCount != 0 {
		t.Fatalf("CurrentCount = %d, want 0", statuses[0].CurrentCount)
	}
}

func TestResumeEmptyID(t *testing.T) {
	svc := NewService(testDeps())
	err := svc.Resume("")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestResumeAlreadyRunning(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("test", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	err = svc.Resume(id)
	if err == nil {
		t.Fatal("expected error for already running scheduler")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ------------------------------------------------------------
// Delete tests
// ------------------------------------------------------------

func TestDeleteRemovesEntry(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("delete-me", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := svc.Delete(id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses after delete, got %d", len(statuses))
	}
}

func TestDeleteEmptyID(t *testing.T) {
	svc := NewService(testDeps())
	err := svc.Delete("")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestDeleteCancelsRunningEntry(t *testing.T) {
	d := testDepsWithPanes("%1")

	ctx, cancel := context.WithCancel(context.Background())
	d.NewContext = func() (context.Context, context.CancelFunc) {
		return ctx, cancel
	}

	svc := NewService(d)
	id, err := svc.Start("delete-cancel", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := svc.Delete(id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify the context was cancelled by Delete (via removeEntry → cancel()).
	select {
	case <-ctx.Done():
		// Expected: context cancelled.
	default:
		t.Fatal("expected context to be cancelled after Delete")
	}
}

// ------------------------------------------------------------
// StopAll tests
// ------------------------------------------------------------

func TestStopAll(t *testing.T) {
	d := testDepsWithPanes("%1", "%2", "%3")
	svc := NewService(d)

	for _, title := range []string{"a", "b", "c"} {
		if _, err := svc.Start(title, "%1", "hello", 60, 1); err != nil {
			t.Fatalf("Start(%q) error = %v", title, err)
		}
	}

	if err := svc.StopAll(); err != nil {
		t.Fatalf("StopAll() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s.Running {
			t.Fatalf("entry %s should be stopped", s.ID)
		}
	}
}

func TestStopAllEmpty(t *testing.T) {
	svc := NewService(testDeps())
	if err := svc.StopAll(); err != nil {
		t.Fatalf("StopAll() error = %v", err)
	}
}

// ------------------------------------------------------------
// Statuses tests
// ------------------------------------------------------------

func TestStatusesEmpty(t *testing.T) {
	svc := NewService(testDeps())
	statuses := svc.Statuses()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestStatusesCopiesData(t *testing.T) {
	d := testDepsWithPanes("%5")
	svc := NewService(d)

	id, err := svc.Start("My Scheduler", "%5", "hello world", 10, 50)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	s := statuses[0]
	if s.ID != id {
		t.Errorf("ID = %q, want %q", s.ID, id)
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
	if s.IntervalSeconds != 10 {
		t.Errorf("IntervalSeconds = %d, want %d", s.IntervalSeconds, 10)
	}
	if s.MaxCount != 50 {
		t.Errorf("MaxCount = %d, want %d", s.MaxCount, 50)
	}
}

func TestStatusesSortedByID(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	var ids []string
	for range 3 {
		id, err := svc.Start("title", "%1", "hello", 60, 1)
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		ids = append(ids, id)
	}

	statuses := svc.Statuses()
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	for i := 1; i < len(statuses); i++ {
		if statuses[i-1].ID >= statuses[i].ID {
			t.Fatalf("statuses not sorted by ID: %q >= %q", statuses[i-1].ID, statuses[i].ID)
		}
	}
}

func TestStopReasonInStatuses(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("reason-test", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := svc.Stop(id); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].StopReason != "stopped" {
		t.Fatalf("StopReason = %q, want %q", statuses[0].StopReason, "stopped")
	}
}

func TestStopReasonClearedOnResume(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	id, err := svc.Start("clear-reason", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := svc.Stop(id); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if err := svc.Resume(id); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].StopReason != "" {
		t.Fatalf("StopReason = %q, want empty after resume", statuses[0].StopReason)
	}
}

func TestEntryStatusFieldCount(t *testing.T) {
	// Guard: if entry or EntryStatus gains/loses a field, this test fails
	// as a reminder to update Statuses() mapping.
	entryFields := reflect.TypeFor[entry]().NumField()
	statusFields := reflect.TypeFor[EntryStatus]().NumField()

	// entry: ID, Title, PaneID, Message, IntervalSeconds, MaxCount,
	// CurrentCount, Running, RunToken, StopReason, cancel = 11 fields
	// (IntervalMinutes was renamed to IntervalSeconds; field count unchanged)
	if entryFields != 11 {
		t.Fatalf("entry has %d fields (expected 11); update Statuses() mapping and this test", entryFields)
	}
	// EntryStatus: ID, Title, PaneID, Message, IntervalSeconds, MaxCount,
	// CurrentCount, Running, StopReason = 9 fields
	// (IntervalMinutes was renamed to IntervalSeconds; field count unchanged)
	if statusFields != 9 {
		t.Fatalf("EntryStatus has %d fields (expected 9); update Statuses() mapping and this test", statusFields)
	}
}

func TestServiceWorksWithoutExplicitEmitter(t *testing.T) {
	d := testDepsWithPanes("%1")
	// Emitter is nil — NewService provides a no-op default.
	svc := NewService(d)

	id, err := svc.Start("default-emitter", "%1", "hello", 60, 1)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Stop triggers emitUpdated which should not panic with default emitter.
	if err := svc.Stop(id); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestNewServicePanicsOnNilDeps(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil required deps")
		}
	}()
	NewService(Deps{})
}

// ------------------------------------------------------------
// runLoop tests
// ------------------------------------------------------------

func TestRunLoopCancellation(t *testing.T) {
	d := testDepsWithPanes("%1")
	svc := NewService(d)

	ctx, cancel := context.WithCancel(context.Background())

	const entryID = "loop-test"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "loop-test", PaneID: "%1", Message: "hello",
		IntervalSeconds: 60, MaxCount: 1, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	var wg sync.WaitGroup
	wg.Go(func() {
		svc.runLoop(ctx, entryID, 1)
	})

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runLoop did not exit after context cancellation")
	}
}

func TestRunLoopPaneGoneEmitsStoppedEvent(t *testing.T) {
	d := testDeps()
	// Pane is gone from the start (CheckPaneAlive returns error).
	d.CheckPaneAlive = func(paneID string) error {
		return fmt.Errorf("pane %s does not exist", paneID)
	}

	var stoppedPayload map[string]string
	d.Emitter = &testEmitter{onEmit: func(name string, payload any) {
		if name == "scheduler:stopped" {
			if p, ok := payload.(map[string]string); ok {
				stoppedPayload = p
			}
		}
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewService(d)

	// Insert entry directly to bypass Start() validation (interval=0 fires immediately).
	const entryID = "gone"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Gone", PaneID: "%9", Message: "hello",
		IntervalSeconds: 0, MaxCount: 1, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	if stoppedPayload["title"] != "Gone" {
		t.Fatalf("stopped title = %q, want %q", stoppedPayload["title"], "Gone")
	}
	if stoppedPayload["reason"] != "target pane is no longer available" {
		t.Fatalf("stopped reason = %q", stoppedPayload["reason"])
	}

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Running {
		t.Fatal("entry should be marked stopped after pane disappearance")
	}
}

func TestRunLoopSendFailureEmitsStoppedEvent(t *testing.T) {
	d := testDepsWithPanes("%1")
	d.SendMessage = func(paneID, message string) error {
		return errors.New("write failed")
	}

	var stoppedPayload map[string]string
	d.Emitter = &testEmitter{onEmit: func(name string, payload any) {
		if name == "scheduler:stopped" {
			if p, ok := payload.(map[string]string); ok {
				stoppedPayload = p
			}
		}
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewService(d)

	const entryID = "send-fail"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Send Fail", PaneID: "%1", Message: "hello",
		IntervalSeconds: 0, MaxCount: 1, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	if stoppedPayload["title"] != "Send Fail" {
		t.Fatalf("stopped title = %q, want %q", stoppedPayload["title"], "Send Fail")
	}
	if !strings.Contains(stoppedPayload["reason"], "message delivery failed") {
		t.Fatalf("stopped reason = %q", stoppedPayload["reason"])
	}
}

func TestRunLoopMaxCountReached(t *testing.T) {
	d := testDepsWithPanes("%1")
	d.SendMessage = func(paneID, message string) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewService(d)

	const entryID = "max-count"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Max Count", PaneID: "%1", Message: "hello",
		IntervalSeconds: 0, MaxCount: 1, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Running {
		t.Fatal("entry should be marked stopped after reaching max count")
	}
}

func TestRunLoopInfiniteCountKeepsEntry(t *testing.T) {
	d := testDepsWithPanes("%1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var svc *Service
	d.SendMessage = func(paneID, message string) error {
		// Set interval to 1 second so the next timer won't fire immediately,
		// then cancel so the loop exits cleanly.
		svc.mu.Lock()
		if e, ok := svc.entries["infinite"]; ok {
			e.IntervalSeconds = 1
		}
		svc.mu.Unlock()
		cancel()
		return nil
	}
	svc = NewService(d)

	const entryID = "infinite"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Infinite", PaneID: "%1", Message: "hello",
		IntervalSeconds: 0, MaxCount: InfiniteCount, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	statuses := svc.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", statuses[0].CurrentCount)
	}
}

func TestRunLoopSkipsWhenPaneBusy(t *testing.T) {
	d := testDepsWithPanes("%1")

	// Pane is busy (not quiet) on first call, quiet on second.
	quietCallCount := 0
	d.IsPaneQuiet = func(paneID string) bool {
		quietCallCount++
		return quietCallCount > 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sendCalled bool
	d.SendMessage = func(paneID, message string) error {
		sendCalled = true
		// Cancel after successful send to exit loop.
		cancel()
		return nil
	}
	var svc *Service
	svc = NewService(d)

	const entryID = "busy-test"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Busy Test", PaneID: "%1", Message: "hello",
		IntervalSeconds: 0, MaxCount: InfiniteCount, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	if quietCallCount < 2 {
		t.Fatalf("IsPaneQuiet called %d times, expected at least 2 (busy skip + quiet send)", quietCallCount)
	}
	if !sendCalled {
		t.Fatal("SendMessage was never called; expected it after pane became quiet")
	}
}

func TestRunLoopSendsWhenPaneQuiet(t *testing.T) {
	d := testDepsWithPanes("%1")
	d.IsPaneQuiet = func(paneID string) bool { return true }

	var sendCalled bool
	d.SendMessage = func(paneID, message string) error {
		sendCalled = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewService(d)

	const entryID = "quiet-test"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Quiet Test", PaneID: "%1", Message: "hello",
		IntervalSeconds: 0, MaxCount: 1, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	if !sendCalled {
		t.Fatal("SendMessage should have been called when pane is quiet")
	}
}

func TestRunLoopDefaultIsPaneQuietNil(t *testing.T) {
	// Verify that nil IsPaneQuiet defaults to always-quiet (SendMessage is called).
	d := testDepsWithPanes("%1")
	d.IsPaneQuiet = nil // explicitly nil

	var sendCalled bool
	d.SendMessage = func(paneID, message string) error {
		sendCalled = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewService(d)

	const entryID = "default-quiet"
	svc.mu.Lock()
	svc.entries[entryID] = &entry{
		ID: entryID, Title: "Default Quiet", PaneID: "%1", Message: "hello",
		IntervalSeconds: 0, MaxCount: 1, Running: true, RunToken: 1, cancel: cancel,
	}
	svc.mu.Unlock()

	svc.runLoop(ctx, entryID, 1)

	if !sendCalled {
		t.Fatal("SendMessage should be called when IsPaneQuiet defaults to always-quiet")
	}
}

// ------------------------------------------------------------
// Template persistence tests
// ------------------------------------------------------------

func TestSaveTemplateValidation(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    Template
		wantErr string
	}{
		{
			name:    "empty title",
			tmpl:    Template{Title: "", Message: "hello", IntervalSeconds: 10, MaxCount: 1},
			wantErr: "title is required",
		},
		{
			name:    "whitespace-only title",
			tmpl:    Template{Title: "   ", Message: "hello", IntervalSeconds: 10, MaxCount: 1},
			wantErr: "title is required",
		},
		{
			name:    "interval zero",
			tmpl:    Template{Title: "test", Message: "hello", IntervalSeconds: 0, MaxCount: 1},
			wantErr: "interval must be at least 10 seconds",
		},
		{
			name:    "max count negative",
			tmpl:    Template{Title: "test", Message: "hello", IntervalSeconds: 10, MaxCount: -1},
			wantErr: "send count must be 0 for infinite or at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := setupTemplateTestService(t)
			err := svc.SaveTemplate("test-session", tt.tmpl)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Errorf("error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestSaveAndLoadTemplate(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	tmpl := Template{
		Title: "Deploy Check", Message: "check deploy status",
		IntervalSeconds: 30, MaxCount: 10,
	}
	if err := svc.SaveTemplate("test-session", tmpl); err != nil {
		t.Fatalf("SaveTemplate() error = %v", err)
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
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
	if loaded[0].IntervalSeconds != 30 {
		t.Errorf("IntervalSeconds = %d, want %d", loaded[0].IntervalSeconds, 30)
	}
	if loaded[0].MaxCount != 10 {
		t.Errorf("MaxCount = %d, want %d", loaded[0].MaxCount, 10)
	}
}

func TestSaveTemplateOverwrite(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	if err := svc.SaveTemplate("test-session", Template{
		Title: "Check", Message: "original", IntervalSeconds: 10, MaxCount: 1,
	}); err != nil {
		t.Fatalf("first save error = %v", err)
	}

	if err := svc.SaveTemplate("test-session", Template{
		Title: "Check", Message: "updated", IntervalSeconds: 10, MaxCount: 50,
	}); err != nil {
		t.Fatalf("second save error = %v", err)
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template (overwrite), got %d", len(loaded))
	}
	if loaded[0].Message != "updated" {
		t.Errorf("Message = %q, want %q", loaded[0].Message, "updated")
	}
	if loaded[0].IntervalSeconds != 10 {
		t.Errorf("IntervalSeconds = %d, want %d", loaded[0].IntervalSeconds, 10)
	}
}

func TestSaveTemplatesSortedByTitle(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	for _, title := range []string{"Zebra", "Alpha", "Middle"} {
		if err := svc.SaveTemplate("test-session", Template{
			Title: title, Message: "msg", IntervalSeconds: 10, MaxCount: 1,
		}); err != nil {
			t.Fatalf("save %q error = %v", title, err)
		}
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(loaded))
	}
	if loaded[0].Title != "Alpha" || loaded[1].Title != "Middle" || loaded[2].Title != "Zebra" {
		t.Errorf("templates not sorted: %v, %v, %v", loaded[0].Title, loaded[1].Title, loaded[2].Title)
	}
}

func TestDeleteTemplate(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	for _, title := range []string{"Keep", "Remove"} {
		if err := svc.SaveTemplate("test-session", Template{
			Title: title, Message: "msg", IntervalSeconds: 10, MaxCount: 1,
		}); err != nil {
			t.Fatalf("save %q error = %v", title, err)
		}
	}

	if err := svc.DeleteTemplate("test-session", "Remove"); err != nil {
		t.Fatalf("DeleteTemplate() error = %v", err)
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].Title != "Keep" {
		t.Errorf("Title = %q, want %q", loaded[0].Title, "Keep")
	}
}

func TestDeleteTemplateEmptyTitle(t *testing.T) {
	svc, _ := setupTemplateTestService(t)
	err := svc.DeleteTemplate("test-session", "")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestDeleteTemplateNonExistent(t *testing.T) {
	svc, _ := setupTemplateTestService(t)
	err := svc.DeleteTemplate("test-session", "ghost")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadTemplatesNoFile(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 templates, got %d", len(loaded))
	}
}

func TestLoadTemplatesMalformedJSON(t *testing.T) {
	svc, tmpDir := setupTemplateTestService(t)

	dir := filepath.Join(tmpDir, templateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, templateFileName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v (expected nil for malformed)", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 templates for malformed JSON, got %d", len(loaded))
	}
}

func TestSaveTemplateMalformedJSONReturnsError(t *testing.T) {
	svc, tmpDir := setupTemplateTestService(t)

	dir := filepath.Join(tmpDir, templateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, templateFileName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := svc.SaveTemplate("test-session", Template{
		Title: "Check", Message: "hello", IntervalSeconds: 10, MaxCount: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "parse templates") {
		t.Fatalf("SaveTemplate() error = %v, want parse templates error", err)
	}
}

func TestDeleteTemplateMalformedJSONReturnsError(t *testing.T) {
	svc, tmpDir := setupTemplateTestService(t)

	dir := filepath.Join(tmpDir, templateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, templateFileName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := svc.DeleteTemplate("test-session", "Check")
	if err == nil || !strings.Contains(err.Error(), "parse templates") {
		t.Fatalf("DeleteTemplate() error = %v, want parse templates error", err)
	}
}

func TestResolveTemplatePathSessionNotFound(t *testing.T) {
	svc := NewService(testDeps())
	_, err := svc.resolveTemplatePath("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestResolveTemplatePathEmptyName(t *testing.T) {
	svc := NewService(testDeps())
	_, err := svc.resolveTemplatePath("")
	if err == nil {
		t.Fatal("expected error for empty session name")
	}
}

func TestTemplateFileFormat(t *testing.T) {
	svc, tmpDir := setupTemplateTestService(t)

	if err := svc.SaveTemplate("test-session", Template{
		Title: "Test", Message: "hello\nworld", IntervalSeconds: 30, MaxCount: 99,
	}); err != nil {
		t.Fatalf("SaveTemplate() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, templateDir, templateFileName))
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

func TestSaveTemplateTitleTrimmed(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	if err := svc.SaveTemplate("test-session", Template{
		Title: "  Padded  ", Message: "msg", IntervalSeconds: 10, MaxCount: 1,
	}); err != nil {
		t.Fatalf("SaveTemplate() error = %v", err)
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].Title != "Padded" {
		t.Errorf("Title = %q, want %q (trimmed)", loaded[0].Title, "Padded")
	}
}

func TestSaveTemplateSessionNameTrimmed(t *testing.T) {
	svc, _ := setupTemplateTestService(t)

	if err := svc.SaveTemplate("  test-session  ", Template{
		Title: "Trimmed Session", Message: "msg", IntervalSeconds: 10, MaxCount: 0,
	}); err != nil {
		t.Fatalf("SaveTemplate() error = %v", err)
	}

	loaded, err := svc.LoadTemplates("test-session")
	if err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 template, got %d", len(loaded))
	}
	if loaded[0].MaxCount != 0 {
		t.Fatalf("MaxCount = %d, want 0", loaded[0].MaxCount)
	}
}

func TestReadTemplatesIOError(t *testing.T) {
	dir := t.TempDir()
	// Point to a directory instead of a file — os.ReadFile on a directory
	// returns an error that is not os.IsNotExist.
	dirAsFile := filepath.Join(dir, "not-a-file")
	if err := os.MkdirAll(dirAsFile, 0o755); err != nil {
		t.Fatal(err)
	}

	// readTemplatesForWrite (allowMalformed=false) should return the error.
	_, err := readTemplatesForWrite(dirAsFile)
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}

	// readTemplates (allowMalformed=true) should also return the error
	// (allowMalformed only affects JSON parse errors, not I/O errors).
	_, err = readTemplates(dirAsFile)
	if err == nil {
		t.Fatal("expected error when path is a directory (even with allowMalformed)")
	}
}

// ------------------------------------------------------------
// Test emitter
// ------------------------------------------------------------

type testEmitter struct {
	onEmit func(name string, payload any)
}

func (e *testEmitter) Emit(name string, payload any) {
	if e.onEmit != nil {
		e.onEmit(name, payload)
	}
}

func (e *testEmitter) EmitWithContext(_ context.Context, name string, payload any) {
	if e.onEmit != nil {
		e.onEmit(name, payload)
	}
}
