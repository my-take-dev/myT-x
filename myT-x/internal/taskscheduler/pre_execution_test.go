package taskscheduler

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/workerutil"

	_ "modernc.org/sqlite"
)

func TestBuildRoleReminderMessage(t *testing.T) {
	t.Parallel()

	message := buildRoleReminderMessage(false)
	if strings.Contains(message, "orchestrated team") {
		t.Fatal("team guidance should not be included for non-team sessions")
	}
	if !strings.Contains(message, "list_agents") {
		t.Fatal("role reminder must mention list_agents")
	}
}

func TestBuildRoleReminderMessage_TeamSession(t *testing.T) {
	t.Parallel()

	message := buildRoleReminderMessage(true)
	if !strings.Contains(message, "orchestrated team") {
		t.Fatal("team guidance should be included for team sessions")
	}
}

func TestPreExecTargetPanes_TaskPanes(t *testing.T) {
	t.Parallel()

	svc := NewService(testDeps())
	items := []QueueItem{
		{TargetPaneID: "%2"},
		{TargetPaneID: "%1"},
		{TargetPaneID: "%2"},
	}

	got, err := svc.preExecTargetPanes(items, QueueConfig{PreExecTargetMode: PreExecTargetModeTaskPanes})
	if err != nil {
		t.Fatalf("preExecTargetPanes returned error: %v", err)
	}
	want := []string{"%2", "%1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preExecTargetPanes = %v, want %v", got, want)
	}
}

func TestPreExecTargetPanes_AllPanes(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.GetSessionPaneIDs = func(sessionName string) ([]string, error) {
		return []string{"%0", "%1", "%0"}, nil
	}
	svc := NewService(deps)

	got, err := svc.preExecTargetPanes(nil, QueueConfig{PreExecTargetMode: PreExecTargetModeAllPanes})
	if err != nil {
		t.Fatalf("preExecTargetPanes returned error: %v", err)
	}
	want := []string{"%0", "%1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preExecTargetPanes = %v, want %v", got, want)
	}
}

func TestPreExecTargetPanes_AllPanesError(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.GetSessionPaneIDs = func(sessionName string) ([]string, error) {
		return nil, context.DeadlineExceeded
	}
	svc := NewService(deps)

	if _, err := svc.preExecTargetPanes(nil, QueueConfig{PreExecTargetMode: PreExecTargetModeAllPanes}); err == nil {
		t.Fatal("expected error from GetSessionPaneIDs")
	}
}

func TestWaitForAllPanesIdle_Immediate(t *testing.T) {
	t.Parallel()

	svc := NewService(testDeps())
	if svc.waitForAllPanesIdle(t.Context(), []string{"%0", "%1"}, 100*time.Millisecond) != idleWaitReady {
		t.Fatal("expected waitForAllPanesIdle to succeed immediately")
	}
}

func TestWaitForAllPanesIdle_BecomesQuietOnNextPoll(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	quiet := false

	deps := testDeps()
	deps.IsPaneQuiet = func(paneID string) bool {
		mu.Lock()
		defer mu.Unlock()
		return quiet
	}
	svc := NewService(deps)

	time.AfterFunc(200*time.Millisecond, func() {
		mu.Lock()
		quiet = true
		mu.Unlock()
	})

	start := time.Now()
	if svc.waitForAllPanesIdle(t.Context(), []string{"%0"}, 5*time.Second) != idleWaitReady {
		t.Fatal("expected waitForAllPanesIdle to succeed after pane becomes quiet")
	}
	if time.Since(start) < preExecIdlePollInterval {
		t.Fatal("expected waitForAllPanesIdle to wait for the next poll")
	}
}

func TestWaitForAllPanesIdle_Timeout(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.IsPaneQuiet = func(paneID string) bool { return false }
	svc := NewService(deps)

	if svc.waitForAllPanesIdle(t.Context(), []string{"%0"}, 100*time.Millisecond) != idleWaitTimedOut {
		t.Fatal("timeout should report idleWaitTimedOut")
	}
}

func TestWaitForAllPanesIdle_ContextCancelled(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.IsPaneQuiet = func(paneID string) bool { return false }
	svc := NewService(deps)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if svc.waitForAllPanesIdle(ctx, []string{"%0"}, time.Second) != idleWaitCanceled {
		t.Fatal("expected waitForAllPanesIdle to stop on cancelled context")
	}
}

func TestWaitForAllPanesIdle_NonPositiveTimeoutWaitsForContextCancellation(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.IsPaneQuiet = func(paneID string) bool { return false }
	svc := NewService(deps)

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(20*time.Millisecond, cancel)

	if svc.waitForAllPanesIdle(ctx, []string{"%0"}, 0) != idleWaitCanceled {
		t.Fatal("expected non-positive timeout to avoid immediate timeout")
	}
}

func TestRunPreExecutionPhase_MultiPaneResetStaggersCommands(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var timestamps []time.Time

	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		return nil
	}
	svc := NewService(deps)
	svc.mu.Lock()
	svc.runStatus = QueuePreparing
	svc.mu.Unlock()

	items := []QueueItem{
		{Title: "t1", Message: "m", TargetPaneID: "%0"},
		{Title: "t2", Message: "m", TargetPaneID: "%1"},
		{Title: "t3", Message: "m", TargetPaneID: "%2"},
	}
	ok := svc.runPreExecutionPhase(t.Context(), items, QueueConfig{
		PreExecTargetMode:  PreExecTargetModeTaskPanes,
		PreExecResetDelay:  0,
		PreExecIdleTimeout: 1,
	})
	if !ok {
		t.Fatal("expected runPreExecutionPhase to succeed")
	}

	mu.Lock()
	ts := slices.Clone(timestamps)
	mu.Unlock()

	if len(ts) != 3 {
		t.Fatalf("expected 3 reset calls, got %d", len(ts))
	}
	// Verify only the lower bound for the inter-pane delay. Scheduler jitter can
	// stretch the gap in busy environments, but it should never shrink it below
	// the reset staggering guard we rely on.
	for i := 1; i < len(ts); i++ {
		gap := ts[i].Sub(ts[i-1])
		if gap < preExecInterPaneResetDelay/2 {
			t.Fatalf("inter-pane reset gap[%d] = %v, want >= %v", i, gap, preExecInterPaneResetDelay/2)
		}
	}
}

func TestRunPreExecutionPhase_ContextCancelDuringResetDelay(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	resetCount := 0

	ctx, cancel := context.WithCancel(t.Context())

	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		mu.Lock()
		resetCount++
		count := resetCount
		mu.Unlock()
		// Cancel after the first send so the inter-pane delay is interrupted.
		if count == 1 {
			cancel()
		}
		return nil
	}
	deps.SendMessagePaste = func(paneID, message string) error {
		t.Fatal("SendMessagePaste should not be called when context is cancelled during reset delay")
		return nil
	}
	svc := NewService(deps)
	svc.mu.Lock()
	svc.runStatus = QueuePreparing
	svc.mu.Unlock()

	items := []QueueItem{
		{Title: "t1", Message: "m", TargetPaneID: "%0"},
		{Title: "t2", Message: "m", TargetPaneID: "%1"},
	}
	ok := svc.runPreExecutionPhase(ctx, items, QueueConfig{
		PreExecTargetMode:  PreExecTargetModeTaskPanes,
		PreExecResetDelay:  0,
		PreExecIdleTimeout: 10,
	})
	if ok {
		t.Fatal("expected runPreExecutionPhase to return false on context cancel during reset delay")
	}

	mu.Lock()
	got := resetCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected 1 reset call before cancel, got %d", got)
	}
}

func TestRunPreExecutionPhase_ContextCancelDuringReminderDelay(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	reminderCount := 0

	ctx, cancel := context.WithCancel(t.Context())

	deps := testDeps()
	deps.SendMessagePaste = func(paneID, message string) error {
		mu.Lock()
		reminderCount++
		count := reminderCount
		mu.Unlock()
		// Cancel after the first reminder so the inter-pane delay is interrupted.
		if count == 1 {
			cancel()
		}
		return nil
	}
	svc := NewService(deps)
	svc.mu.Lock()
	svc.runStatus = QueuePreparing
	svc.mu.Unlock()

	items := []QueueItem{
		{Title: "t1", Message: "m", TargetPaneID: "%0"},
		{Title: "t2", Message: "m", TargetPaneID: "%1"},
	}
	ok := svc.runPreExecutionPhase(ctx, items, QueueConfig{
		PreExecTargetMode:  PreExecTargetModeTaskPanes,
		PreExecResetDelay:  0,
		PreExecIdleTimeout: 10,
	})
	if ok {
		t.Fatal("expected runPreExecutionPhase to return false on context cancel during reminder delay")
	}

	mu.Lock()
	got := reminderCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected 1 reminder call before cancel, got %d", got)
	}
}

func TestRunPreExecutionPhase_FailsWhenAllResetsFail(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.SendClearCommand = func(paneID, command string) error {
		return context.DeadlineExceeded
	}
	deps.SendMessagePaste = func(paneID, message string) error {
		t.Fatal("SendMessagePaste should not be called when all resets fail")
		return nil
	}
	svc := NewService(deps)
	svc.mu.Lock()
	svc.runStatus = QueuePreparing
	svc.mu.Unlock()

	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	ok := svc.runPreExecutionPhase(t.Context(), items, QueueConfig{
		PreExecTargetMode:  PreExecTargetModeTaskPanes,
		PreExecResetDelay:  0,
		PreExecIdleTimeout: 10,
	})

	if ok {
		t.Fatal("expected runPreExecutionPhase to fail when all resets fail")
	}
	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Fatalf("expected queue to return to idle, got %q", status.RunStatus)
	}
}

func TestRunPreExecutionPhase_FailsWhenAllRemindersFail(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	deps.SendMessagePaste = func(paneID, message string) error {
		return context.DeadlineExceeded
	}
	svc := NewService(deps)
	svc.mu.Lock()
	svc.runStatus = QueuePreparing
	svc.mu.Unlock()

	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	ok := svc.runPreExecutionPhase(t.Context(), items, QueueConfig{
		PreExecTargetMode:  PreExecTargetModeTaskPanes,
		PreExecResetDelay:  0,
		PreExecIdleTimeout: 10,
	})

	if ok {
		t.Fatal("expected runPreExecutionPhase to fail when all reminders fail")
	}
	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Fatalf("expected queue to return to idle, got %q", status.RunStatus)
	}
}

func TestStart_PreExecutionOrder(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "orchestrator.db")
	prepareTaskSchedulerDB(t, dbPath)

	var mu sync.Mutex
	events := make([]string, 0, 4)

	deps := testDeps()
	deps.ResolveOrchestratorDBPath = func() (string, error) { return dbPath, nil }
	deps.IsAgentTeamSession = func(sessionName string) bool { return true }
	deps.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		go fn(ctx)
	}
	deps.SendClearCommand = func(paneID, command string) error {
		mu.Lock()
		events = append(events, "clear:"+paneID+":"+command)
		mu.Unlock()
		return nil
	}
	deps.SendMessagePaste = func(paneID, message string) error {
		mu.Lock()
		switch {
		case strings.Contains(message, "Use the orchestrator MCP to confirm your role"):
			events = append(events, "reminder:"+paneID)
		case strings.Contains(message, "【タスク完了時の応答方法】"):
			events = append(events, "task:"+paneID)
			mu.Unlock()
			markLatestTaskCompleted(t, dbPath)
			return nil
		default:
			events = append(events, "message:"+paneID)
		}
		mu.Unlock()
		return nil
	}

	svc := NewService(deps)
	items := []QueueItem{{
		Title:        "task1",
		Message:      "do work",
		TargetPaneID: "%0",
	}}
	config := QueueConfig{
		PreExecEnabled:     true,
		PreExecTargetMode:  PreExecTargetModeTaskPanes,
		PreExecResetDelay:  1,
		PreExecIdleTimeout: 1,
	}

	if err := svc.Start(config, items); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	deadline := time.After(15 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for queue completion")
		default:
		}
		status := svc.GetStatus()
		if status.RunStatus == QueueCompleted {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	mu.Lock()
	got := slices.Clone(events)
	mu.Unlock()

	wantPrefix := []string{"clear:%0:/new", "reminder:%0", "task:%0"}
	if len(got) < len(wantPrefix) {
		t.Fatalf("event order too short: got %v", got)
	}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("unexpected pre-exec order: got %v, want prefix %v", got, wantPrefix)
	}
}

func prepareTaskSchedulerDB(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite db: %v", err)
		}
	}()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			name TEXT PRIMARY KEY,
			pane_id TEXT NOT NULL,
			role TEXT NOT NULL,
			skills TEXT NOT NULL,
			created_at TEXT NOT NULL,
			mcp_instance_id TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS send_messages (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			task_id TEXT PRIMARY KEY,
			agent_name TEXT NOT NULL,
			assignee_pane_id TEXT NOT NULL,
			sender_pane_id TEXT NOT NULL,
			sender_name TEXT NOT NULL,
			sender_instance_id TEXT NOT NULL,
			send_message_id TEXT NOT NULL,
			status TEXT NOT NULL,
			sent_at TEXT NOT NULL,
			completed_at TEXT DEFAULT '',
			is_now_session INTEGER NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("prepare scheduler db: %v", err)
		}
	}
}

func markLatestTaskCompleted(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite db: %v", err)
		}
	}()

	var taskID string
	err = db.QueryRow(`SELECT task_id FROM tasks ORDER BY sent_at DESC, task_id DESC LIMIT 1`).Scan(&taskID)
	if err != nil {
		t.Fatalf("query latest task: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE tasks SET status = 'completed', completed_at = ? WHERE task_id = ?`,
		time.Now().UTC().Format(time.RFC3339),
		taskID,
	); err != nil {
		t.Fatalf("mark task completed: %v", err)
	}
}
