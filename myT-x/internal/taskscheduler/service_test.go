package taskscheduler

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/workerutil"
)

// testDeps returns a Deps with all required fields set to safe defaults.
func testDeps() Deps {
	return Deps{
		Emitter:        apptypes.NoopEmitter{},
		IsShuttingDown: func() bool { return false },
		CheckPaneAlive: func(paneID string) error { return nil },
		SendMessagePaste: func(paneID, message string) error {
			return nil
		},
		ResolveOrchestratorDBPath: func() (string, error) {
			return ":memory:", nil
		},
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
		LaunchWorker: func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
			go fn(ctx)
		},
		BaseRecoveryOptions: func() workerutil.RecoveryOptions {
			return workerutil.RecoveryOptions{MaxRetries: 0}
		},
		SendClearCommand: func(paneID, command string) error { return nil },
		SessionName:      "test-session",
	}
}

func TestNewService_PanicsOnNilRequiredFields(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil required fields, got none")
		}
	}()
	NewService(Deps{})
}

func TestNewService_DefaultsOptionalFields(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.Emitter = nil
	deps.IsShuttingDown = nil
	svc := NewService(deps)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestGetStatus_InitialState(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())
	status := svc.GetStatus()

	if status.RunStatus != QueueIdle {
		t.Errorf("expected run_status=%q, got %q", QueueIdle, status.RunStatus)
	}
	if status.CurrentIndex != -1 {
		t.Errorf("expected current_index=-1, got %d", status.CurrentIndex)
	}
	if status.Items == nil {
		t.Fatal("expected non-nil items slice")
	}
	if len(status.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(status.Items))
	}
}

func TestStart_EmptyItems(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())
	err := svc.Start(QueueConfig{}, nil)
	if err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestStart_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []QueueItem
	}{
		{
			name:  "empty title",
			items: []QueueItem{{Title: "", Message: "msg", TargetPaneID: "%0"}},
		},
		{
			name:  "empty message",
			items: []QueueItem{{Title: "task1", Message: "", TargetPaneID: "%0"}},
		},
		{
			name:  "empty target pane",
			items: []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewService(testDeps())
			err := svc.Start(QueueConfig{}, tt.items)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestStart_AlreadyRunning(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	// Block the worker to keep state as running.
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go func() {
			<-ctx.Done()
		}()
	}
	svc := NewService(deps)

	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	// Force status to running.
	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.mu.Unlock()

	err := svc.Start(QueueConfig{}, items)
	if err == nil {
		t.Fatal("expected error when starting already running queue")
	}

	// Cleanup.
	svc.StopAll()
}

func TestStop_NotRunning(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())
	err := svc.Stop()
	if err == nil {
		t.Fatal("expected error when stopping idle queue")
	}
}

func TestPauseResume(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go func() {
			<-ctx.Done()
		}()
	}
	svc := NewService(deps)

	// Pause when not running.
	if err := svc.Pause(); err == nil {
		t.Fatal("expected error on pause when idle")
	}

	// Resume when not paused.
	if err := svc.Resume(); err == nil {
		t.Fatal("expected error on resume when idle")
	}

	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.mu.Unlock()

	if err := svc.Pause(); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if svc.GetStatus().RunStatus != QueuePaused {
		t.Errorf("expected paused, got %s", svc.GetStatus().RunStatus)
	}

	svc.StopAll()
}

func TestAddItem(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	tests := []struct {
		name    string
		title   string
		message string
		paneID  string
		wantErr bool
	}{
		{"valid", "task1", "do something", "%0", false},
		{"empty title", "", "msg", "%0", true},
		{"empty message", "task1", "", "%0", true},
		{"empty pane", "task1", "msg", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			localSvc := NewService(testDeps())
			err := localSvc.AddItem(tt.title, tt.message, tt.paneID, false, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("AddItem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify the added item.
	if err := svc.AddItem("task1", "do something", "%0", false, ""); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	status := svc.GetStatus()
	if len(status.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(status.Items))
	}
	if status.Items[0].Title != "task1" {
		t.Errorf("expected title 'task1', got %q", status.Items[0].Title)
	}
	if status.Items[0].Status != ItemStatusPending {
		t.Errorf("expected status %q, got %q", ItemStatusPending, status.Items[0].Status)
	}
}

func TestRemoveItem(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.AddItem("task1", "msg1", "%0", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddItem("task2", "msg2", "%1", false, ""); err != nil {
		t.Fatal(err)
	}

	items := svc.GetStatus().Items
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Remove the first item.
	if err := svc.RemoveItem(items[0].ID); err != nil {
		t.Fatalf("RemoveItem failed: %v", err)
	}

	status := svc.GetStatus()
	if len(status.Items) != 1 {
		t.Fatalf("expected 1 item after removal, got %d", len(status.Items))
	}
	if status.Items[0].Title != "task2" {
		t.Errorf("expected remaining item title 'task2', got %q", status.Items[0].Title)
	}
	if status.Items[0].OrderIndex != 0 {
		t.Errorf("expected reindexed order_index=0, got %d", status.Items[0].OrderIndex)
	}
}

func TestRemoveItem_Errors(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.RemoveItem(""); err == nil {
		t.Fatal("expected error for empty id")
	}
	if err := svc.RemoveItem("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent id")
	}
}

func TestReorderItems(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.AddItem("task1", "msg1", "%0", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddItem("task2", "msg2", "%1", false, ""); err != nil {
		t.Fatal(err)
	}

	items := svc.GetStatus().Items
	// Reverse order.
	err := svc.ReorderItems([]string{items[1].ID, items[0].ID})
	if err != nil {
		t.Fatalf("ReorderItems failed: %v", err)
	}

	reordered := svc.GetStatus().Items
	if reordered[0].Title != "task2" {
		t.Errorf("expected first item 'task2', got %q", reordered[0].Title)
	}
	if reordered[1].Title != "task1" {
		t.Errorf("expected second item 'task1', got %q", reordered[1].Title)
	}
}

func TestReorderItems_Errors(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.ReorderItems(nil); err == nil {
		t.Fatal("expected error for empty ids")
	}
	if err := svc.AddItem("task1", "msg1", "%0", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReorderItems([]string{"bad-id"}); err == nil {
		t.Fatal("expected error for mismatched ids")
	}
}

func TestUpdateItem(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.AddItem("task1", "msg1", "%0", false, ""); err != nil {
		t.Fatal(err)
	}

	id := svc.GetStatus().Items[0].ID
	if err := svc.UpdateItem(id, "updated", "new msg", "%1", false, ""); err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	updated := svc.GetStatus().Items[0]
	if updated.Title != "updated" {
		t.Errorf("expected title 'updated', got %q", updated.Title)
	}
	if updated.Message != "new msg" {
		t.Errorf("expected message 'new msg', got %q", updated.Message)
	}
	if updated.TargetPaneID != "%1" {
		t.Errorf("expected pane '%%1', got %q", updated.TargetPaneID)
	}
}

func TestUpdateItem_Errors(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.UpdateItem("", "t", "m", "%0", false, ""); err == nil {
		t.Fatal("expected error for empty id")
	}
	if err := svc.UpdateItem("x", "", "m", "%0", false, ""); err == nil {
		t.Fatal("expected error for empty title")
	}
	if err := svc.UpdateItem("x", "t", "", "%0", false, ""); err == nil {
		t.Fatal("expected error for empty message")
	}
	if err := svc.UpdateItem("x", "t", "m", "", false, ""); err == nil {
		t.Fatal("expected error for empty pane")
	}
}

func TestStopAll_Idempotent(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())
	// Should not panic when called on idle service.
	svc.StopAll()
	svc.StopAll()
}

func TestBuildTaskResponseInstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		taskID   string
		contains string
	}{
		{"with task id", "t-abc123", "t-abc123"},
		{"empty task id", "", "<task_id>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := buildTaskResponseInstruction(tt.taskID)
			if len(result) == 0 {
				t.Fatal("expected non-empty instruction")
			}
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected instruction to contain %q", tt.contains)
			}
			if !strings.Contains(result, "send_response") {
				t.Error("expected instruction to contain 'send_response'")
			}
		})
	}
}

func TestQueueStatus_ItemsCopy(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())
	if err := svc.AddItem("task1", "msg", "%0", false, ""); err != nil {
		t.Fatal(err)
	}

	status1 := svc.GetStatus()
	status2 := svc.GetStatus()

	// Mutating status1.Items should not affect status2.Items.
	status1.Items[0].Title = "mutated"
	if status2.Items[0].Title == "mutated" {
		t.Error("GetStatus should return a copy, not a reference to internal state")
	}
}

// TestQueueItemFieldCount guards against struct field additions without
// updating serialization and test code.
func TestQueueItemFieldCount(t *testing.T) {
	t.Parallel()
	expected := 13
	actual := reflect.TypeFor[QueueItem]().NumField()
	if actual != expected {
		t.Errorf("QueueItem has %d fields, expected %d. Update tests and serialization if fields were added.", actual, expected)
	}
}

// TestQueueConfigFieldCount guards against struct field additions.
func TestQueueConfigFieldCount(t *testing.T) {
	t.Parallel()
	expected := 0
	actual := reflect.TypeFor[QueueConfig]().NumField()
	if actual != expected {
		t.Errorf("QueueConfig has %d fields, expected %d. Update tests if fields were added.", actual, expected)
	}
}

func TestQueueStatusFieldCount(t *testing.T) {
	t.Parallel()
	expected := 5
	actual := reflect.TypeFor[QueueStatus]().NumField()
	if actual != expected {
		t.Errorf("QueueStatus has %d fields, expected %d.", actual, expected)
	}
}

func TestReorderItems_DuplicateIDs(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())
	if err := svc.AddItem("task1", "msg1", "%0", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddItem("task2", "msg2", "%1", false, ""); err != nil {
		t.Fatal(err)
	}
	items := svc.GetStatus().Items
	err := svc.ReorderItems([]string{items[0].ID, items[0].ID})
	if err == nil {
		t.Fatal("expected error for duplicate ids")
	}
}

func TestEmitEvents(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var events []string

	deps := testDeps()
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, payload any) {
		mu.Lock()
		events = append(events, name)
		mu.Unlock()
	})
	svc := NewService(deps)

	if err := svc.AddItem("task1", "msg", "%0", false, ""); err != nil {
		t.Fatal(err)
	}

	// Wait briefly for async event emission.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Error("expected at least one event to be emitted")
	}
	found := slices.Contains(events, "task-scheduler:updated")
	if !found {
		t.Error("expected 'task-scheduler:updated' event")
	}
}

func TestPause_EmitsUpdated(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var events []string

	deps := testDeps()
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, payload any) {
		mu.Lock()
		events = append(events, name)
		mu.Unlock()
	})
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go func() { <-ctx.Done() }()
	}
	svc := NewService(deps)

	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.mu.Unlock()

	// Reset events so we only capture pause events.
	mu.Lock()
	events = nil
	mu.Unlock()

	if err := svc.Pause(); err != nil {
		t.Fatalf("pause failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	found := slices.Contains(events, "task-scheduler:updated")
	if !found {
		t.Error("Pause() should emit 'task-scheduler:updated' event")
	}

	svc.StopAll()
}

func TestStop_MarksRunningItemAsSkipped(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go func() { <-ctx.Done() }()
	}
	svc := NewService(deps)

	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Simulate an item in running state.
	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.items[0].Status = ItemStatusRunning
	svc.mu.Unlock()

	if err := svc.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	status := svc.GetStatus()
	if status.Items[0].Status != ItemStatusSkipped {
		t.Errorf("expected running item to be skipped after Stop, got %q", status.Items[0].Status)
	}
}

func TestRunLoop_ResumesRunningItem(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go fn(ctx)
	}
	svc := NewService(deps)

	// Set up in-memory orchestrator DB with a completed task.
	db, err := openOrchestratorDB(":memory:")
	if err != nil {
		t.Fatalf("open orc db: %v", err)
	}
	if _, err := db.db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		task_id TEXT PRIMARY KEY,
		agent_name TEXT NOT NULL,
		status TEXT NOT NULL,
		sent_at TEXT NOT NULL,
		completed_at TEXT DEFAULT ''
	)`); err != nil {
		t.Fatalf("create tasks table: %v", err)
	}
	if _, err := db.db.Exec(
		`INSERT INTO tasks (task_id, agent_name, status, sent_at, completed_at) VALUES (?, ?, ?, ?, ?)`,
		"orc-task-1", "task-master", "completed", "2026-01-01T00:00:00Z", "2026-01-01T00:01:00Z",
	); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	svc.dbMu.Lock()
	svc.orcDB = db
	svc.dbMu.Unlock()

	// Simulate a running item (as if resuming after Pause).
	svc.mu.Lock()
	svc.items = []QueueItem{
		{
			ID:           "item-1",
			Title:        "task1",
			Message:      "msg",
			TargetPaneID: "%0",
			Status:       ItemStatusRunning,
			OrcTaskID:    "orc-task-1",
		},
	}
	svc.runStatus = QueueRunning
	svc.currentIndex = 0
	svc.mu.Unlock()

	ctx := t.Context()
	svc.launchWorker(ctx)

	// Wait for item to complete (poll interval is 10s).
	deadline := time.After(15 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for running item to complete")
		default:
		}
		s := svc.GetStatus()
		if s.Items[0].Status == ItemStatusCompleted {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	status := svc.GetStatus()
	if status.RunStatus != QueueCompleted {
		t.Errorf("expected queue completed, got %q", status.RunStatus)
	}

	svc.StopAll()
}

func TestGetStatus_IncludesSessionName(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.SessionName = "my-session"
	svc := NewService(deps)

	status := svc.GetStatus()
	if status.SessionName != "my-session" {
		t.Errorf("expected session_name=%q, got %q", "my-session", status.SessionName)
	}
}

func TestAddItem_ClearBefore(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.AddItem("task1", "msg", "%0", true, "/clear"); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	item := svc.GetStatus().Items[0]
	if !item.ClearBefore {
		t.Error("expected ClearBefore=true")
	}
	if item.ClearCommand != "/clear" {
		t.Errorf("expected ClearCommand=/clear, got %q", item.ClearCommand)
	}

	if err := svc.AddItem("task2", "msg", "%1", false, ""); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	item2 := svc.GetStatus().Items[1]
	if item2.ClearBefore {
		t.Error("expected ClearBefore=false")
	}
}

func TestUpdateItem_ClearBefore(t *testing.T) {
	t.Parallel()
	svc := NewService(testDeps())

	if err := svc.AddItem("task1", "msg", "%0", false, ""); err != nil {
		t.Fatal(err)
	}
	id := svc.GetStatus().Items[0].ID

	if err := svc.UpdateItem(id, "task1", "msg", "%0", true, "/reset"); err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	item := svc.GetStatus().Items[0]
	if !item.ClearBefore {
		t.Error("expected ClearBefore=true after update")
	}
	if item.ClearCommand != "/reset" {
		t.Errorf("expected ClearCommand=/reset, got %q", item.ClearCommand)
	}
}

func TestStart_CopiesClearBefore(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go func() { <-ctx.Done() }()
	}
	svc := NewService(deps)

	items := []QueueItem{
		{Title: "task1", Message: "msg", TargetPaneID: "%0", ClearBefore: true, ClearCommand: "/new"},
		{Title: "task2", Message: "msg", TargetPaneID: "%1", ClearBefore: false},
	}
	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	status := svc.GetStatus()
	if !status.Items[0].ClearBefore {
		t.Error("expected first item ClearBefore=true")
	}
	if status.Items[0].ClearCommand != "/new" {
		t.Errorf("expected first item ClearCommand=/new, got %q", status.Items[0].ClearCommand)
	}
	if status.Items[1].ClearBefore {
		t.Error("expected second item ClearBefore=false")
	}

	svc.StopAll()
}

func TestExecuteClearPreStep_SendsClearCommand(t *testing.T) {
	t.Parallel()

	var capturedPaneID, capturedCmd string
	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		capturedPaneID = paneID
		capturedCmd = command
		return nil
	}
	svc := NewService(deps)

	ctx := t.Context()
	item := QueueItem{TargetPaneID: "%0", ClearBefore: true, ClearCommand: "/reset"}
	ok := svc.executeClearPreStep(ctx, item)

	if !ok {
		t.Fatal("expected executeClearPreStep to return true")
	}
	if capturedPaneID != "%0" {
		t.Errorf("expected pane %%0, got %q", capturedPaneID)
	}
	if capturedCmd != "/reset" {
		t.Errorf("expected command /reset, got %q", capturedCmd)
	}
}

func TestExecuteClearPreStep_DefaultCommand(t *testing.T) {
	t.Parallel()

	var capturedCmd string
	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		capturedCmd = command
		return nil
	}
	svc := NewService(deps)

	ctx := t.Context()
	item := QueueItem{TargetPaneID: "%0", ClearBefore: true, ClearCommand: ""}
	svc.executeClearPreStep(ctx, item)

	if capturedCmd != "/new" {
		t.Errorf("expected default command /new, got %q", capturedCmd)
	}
}

func TestExecuteClearPreStep_SkipWhenFalse(t *testing.T) {
	t.Parallel()

	called := false
	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		called = true
		return nil
	}
	svc := NewService(deps)

	ctx := t.Context()
	item := QueueItem{TargetPaneID: "%0", ClearBefore: false}
	ok := svc.executeClearPreStep(ctx, item)

	if !ok {
		t.Fatal("expected executeClearPreStep to return true")
	}
	if called {
		t.Error("SendClearCommand should NOT be called when ClearBefore=false")
	}
}

func TestExecuteClearPreStep_ClearFailureContinues(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		return errors.New("send failed")
	}
	svc := NewService(deps)

	ctx := t.Context()
	item := QueueItem{TargetPaneID: "%0", ClearBefore: true, ClearCommand: "/new"}
	ok := svc.executeClearPreStep(ctx, item)

	if !ok {
		t.Fatal("expected executeClearPreStep to return true even on failure")
	}
}

func TestExecuteClearPreStep_ContextCancelled(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error { return nil }
	svc := NewService(deps)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	item := QueueItem{TargetPaneID: "%0", ClearBefore: true, ClearCommand: "/new"}
	ok := svc.executeClearPreStep(ctx, item)

	if ok {
		t.Fatal("expected executeClearPreStep to return false on cancelled context")
	}
}

func TestDepsFieldCount(t *testing.T) {
	t.Parallel()
	expected := 10
	actual := reflect.TypeFor[Deps]().NumField()
	if actual != expected {
		t.Errorf("Deps has %d fields, expected %d. Update validateRequired and testDeps if fields were added.", actual, expected)
	}
}

func TestEmitStopped_IncludesSessionName(t *testing.T) {
	t.Parallel()

	var capturedPayload any

	deps := testDeps()
	deps.SessionName = "ses-abc"
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, payload any) {
		if name == "task-scheduler:stopped" {
			capturedPayload = payload
		}
	})
	svc := NewService(deps)

	svc.emitStopped("test reason")

	payloadMap, ok := capturedPayload.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string payload, got %T", capturedPayload)
	}
	if payloadMap["session_name"] != "ses-abc" {
		t.Errorf("expected session_name=%q, got %q", "ses-abc", payloadMap["session_name"])
	}
	if payloadMap["reason"] != "test reason" {
		t.Errorf("expected reason=%q, got %q", "test reason", payloadMap["reason"])
	}
}
