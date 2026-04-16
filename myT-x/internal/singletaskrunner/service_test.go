package singletaskrunner

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/workerutil"
)

type recordedEvent struct {
	name    string
	payload any
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) emit(name string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{name: name, payload: payload})
}

func (r *eventRecorder) hasEvent(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range r.events {
		if event.name == name {
			return true
		}
	}
	return false
}

func (r *eventRecorder) countEvents(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, event := range r.events {
		if event.name == name {
			count++
		}
	}
	return count
}

func testDeps(sentCh chan string, recorder *eventRecorder) Deps {
	return Deps{
		Emitter: apptypes.EventEmitterFunc(func(name string, payload any) {
			if recorder != nil {
				recorder.emit(name, payload)
			}
		}),
		IsShuttingDown: func() bool { return false },
		CheckPaneAlive: func(string) error { return nil },
		SendMessagePaste: func(_ string, message string) error {
			if sentCh != nil {
				sentCh <- message
			}
			return nil
		},
		SendClearCommand: func(string, string) error { return nil },
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
		LaunchWorker: func(_ string, ctx context.Context, fn func(ctx context.Context), _ workerutil.RecoveryOptions) {
			go fn(ctx)
		},
		BaseRecoveryOptions: func() workerutil.RecoveryOptions {
			return workerutil.RecoveryOptions{MaxRetries: 0}
		},
		SessionName: "session-a",
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, description string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition %q was not met before timeout", description)
}

func mustAddItem(t *testing.T, svc *Service, title, message, targetPaneID string, clearBefore bool, clearCommand string) {
	t.Helper()
	if err := svc.AddItem(title, message, targetPaneID, clearBefore, clearCommand); err != nil {
		t.Fatalf("AddItem(%q): %v", title, err)
	}
}

func mustSetClearDelay(t *testing.T, svc *Service, delaySec int) {
	t.Helper()
	if err := svc.SetClearDelay(delaySec); err != nil {
		t.Fatalf("SetClearDelay(%d): %v", delaySec, err)
	}
}

func mustStart(t *testing.T, svc *Service) {
	t.Helper()
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func mustCompleteTask(t *testing.T, svc *Service, taskID, result string) {
	t.Helper()
	if err := svc.CompleteTask(taskID, result); err != nil {
		t.Fatalf("CompleteTask(%s): %v", taskID, err)
	}
}

func TestNewServicePanicsOnNilRequiredFields(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil required fields, got none")
		}
	}()

	NewService(Deps{})
}

func TestNewServicePanicsOnEmptySessionName(t *testing.T) {
	t.Parallel()

	deps := testDeps(nil, nil)
	deps.SessionName = ""

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty session name, got none")
		}
	}()

	NewService(deps)
}

func TestSetRunStateLockedReturnsNoopCancelWhenUnset(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	svc.runStatus = QueueRunning
	svc.currentIndex = 3

	svc.mu.Lock()
	cancel := svc.setRunStateLocked(QueueCompleted)
	if svc.runStatus != QueueCompleted {
		t.Fatalf("runStatus = %q, want %q", svc.runStatus, QueueCompleted)
	}
	if svc.currentIndex != -1 {
		t.Fatalf("currentIndex = %d, want -1", svc.currentIndex)
	}
	if svc.cancel != nil {
		t.Fatal("cancel should be cleared")
	}
	svc.mu.Unlock()

	cancel()
}

func TestResetRunStateLockedReturnsActiveCancelFunc(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	called := false
	svc.runStatus = QueueRunning
	svc.currentIndex = 1
	svc.cancel = func() { called = true }

	svc.mu.Lock()
	cancel := svc.resetRunStateLocked()
	if svc.runStatus != QueueIdle {
		t.Fatalf("runStatus = %q, want %q", svc.runStatus, QueueIdle)
	}
	if svc.currentIndex != -1 {
		t.Fatalf("currentIndex = %d, want -1", svc.currentIndex)
	}
	if svc.cancel != nil {
		t.Fatal("cancel should be cleared")
	}
	svc.mu.Unlock()

	cancel()
	if !called {
		t.Fatal("returned cancel func should invoke the active cancel")
	}
}

func TestSingleTaskRunnerSnapshotFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[QueueItem]().NumField(); got != 13 {
		t.Fatalf("QueueItem field count = %d, want 13; update this test when QueueItem changes", got)
	}
	if got := reflect.TypeFor[QueueStatus]().NumField(); got != 7 {
		t.Fatalf("QueueStatus field count = %d, want 7; update this test when QueueStatus changes", got)
	}
}

func TestStartRotatesGenerationAndIgnoresStaleFailure(t *testing.T) {
	deps := testDeps(nil, nil)
	deps.LaunchWorker = func(_ string, _ context.Context, _ func(ctx context.Context), _ workerutil.RecoveryOptions) {}

	svc := NewService(deps)
	if err := svc.AddItem("Task 1", "message", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	initialGeneration := svc.GetStatus().GenerationID
	if err := svc.Start(); err != nil {
		t.Fatalf("Start(first): %v", err)
	}
	firstRun := svc.GetStatus()
	if firstRun.GenerationID == initialGeneration {
		t.Fatal("expected Start to rotate generation_id")
	}

	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start(second): %v", err)
	}
	secondRun := svc.GetStatus()
	if secondRun.GenerationID == firstRun.GenerationID {
		t.Fatal("expected each run to use a distinct generation_id")
	}

	svc.failItem(firstRun.GenerationID, 0, secondRun.Items[0].ID, "stale failure")

	current := svc.GetStatus()
	if current.RunStatus != QueueRunning {
		t.Fatalf("RunStatus after stale failure = %q, want %q", current.RunStatus, QueueRunning)
	}
	if current.GenerationID != secondRun.GenerationID {
		t.Fatalf("GenerationID after stale failure = %q, want %q", current.GenerationID, secondRun.GenerationID)
	}
	if current.Items[0].Status != ItemStatusPending {
		t.Fatalf("Item status after stale failure = %q, want %q", current.Items[0].Status, ItemStatusPending)
	}
}

func TestServiceRetireSuppressesEvents(t *testing.T) {
	recorder := &eventRecorder{}
	svc := NewService(testDeps(nil, recorder))

	svc.Retire()
	svc.emitUpdated()
	svc.emitStopped("retired", "retired-session", "retired-generation")

	if recorder.hasEvent("single-task-runner:updated") {
		t.Fatal("updated event should be suppressed after Retire")
	}
	if recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("stopped event should be suppressed after Retire")
	}
}

func TestEmitStopped_UsesProvidedSnapshot(t *testing.T) {
	t.Parallel()

	var capturedPayload any

	deps := testDeps(nil, nil)
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, payload any) {
		if name == "single-task-runner:stopped" {
			capturedPayload = payload
		}
	})
	svc := NewService(deps)
	svc.mu.Lock()
	svc.sessionName = "ses-current"
	svc.generationID = "gen-current"
	svc.mu.Unlock()

	svc.emitStopped("retired", "ses-snap", "gen-snap")

	payloadMap, ok := capturedPayload.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string payload, got %T", capturedPayload)
	}
	if payloadMap["session_name"] != "ses-snap" {
		t.Fatalf("session_name = %q, want %q", payloadMap["session_name"], "ses-snap")
	}
	if payloadMap["generation_id"] != "gen-snap" {
		t.Fatalf("generation_id = %q, want %q", payloadMap["generation_id"], "gen-snap")
	}
	if payloadMap["reason"] != "retired" {
		t.Fatalf("reason = %q, want %q", payloadMap["reason"], "retired")
	}
}

func TestServiceRetiredRejectsPublicEntryPoints(t *testing.T) {
	tests := []struct {
		name   string
		action func(*Service, string) error
	}{
		{
			name: "start",
			action: func(svc *Service, _ string) error {
				return svc.Start()
			},
		},
		{
			name: "stop",
			action: func(svc *Service, _ string) error {
				return svc.Stop()
			},
		},
		{
			name: "add item",
			action: func(svc *Service, _ string) error {
				return svc.AddItem("Task 2", "message", "%1", false, "")
			},
		},
		{
			name: "enqueue tasks",
			action: func(svc *Service, _ string) error {
				_, err := svc.EnqueueTasks("%1", []EnqueueTaskInput{{Title: "Task 3", Message: "message"}})
				return err
			},
		},
		{
			name: "remove item",
			action: func(svc *Service, taskID string) error {
				return svc.RemoveItem(taskID)
			},
		},
		{
			name: "reorder items",
			action: func(svc *Service, taskID string) error {
				return svc.ReorderItems([]string{taskID})
			},
		},
		{
			name: "update item",
			action: func(svc *Service, taskID string) error {
				return svc.UpdateItem(taskID, "Updated", "message", "%1", false, "")
			},
		},
		{
			name: "set clear delay",
			action: func(svc *Service, _ string) error {
				return svc.SetClearDelay(0)
			},
		},
		{
			name: "complete task",
			action: func(svc *Service, taskID string) error {
				return svc.CompleteTask(taskID, "done")
			},
		},
		{
			name: "fail task",
			action: func(svc *Service, taskID string) error {
				return svc.FailTask(taskID, "failed")
			},
		},
		{
			name: "cancel task",
			action: func(svc *Service, taskID string) error {
				return svc.CancelTask(taskID, "cancelled")
			},
		},
		{
			name: "rename session",
			action: func(svc *Service, _ string) error {
				return svc.RenameSession("renamed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &eventRecorder{}
			svc := NewService(testDeps(nil, recorder))
			if err := svc.AddItem("Task 1", "message", "%1", false, ""); err != nil {
				t.Fatalf("AddItem(setup): %v", err)
			}
			taskID := svc.GetStatus().Items[0].ID
			recorder.mu.Lock()
			recorder.events = nil
			recorder.mu.Unlock()

			svc.Retire()

			if err := tt.action(svc, taskID); !errors.Is(err, errServiceRetired) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, errServiceRetired)
			}

			status := svc.GetStatus()
			if len(status.Items) != 0 {
				t.Fatalf("Items count = %d, want 0 for retired snapshot", len(status.Items))
			}
			if status.SessionName != "" {
				t.Fatalf("SessionName = %q, want empty for retired snapshot", status.SessionName)
			}
			if status.ClearDelaySec != DefaultClearDelay {
				t.Fatalf("ClearDelaySec = %d, want %d", status.ClearDelaySec, DefaultClearDelay)
			}
			if got := svc.GetClearDelay(); got != DefaultClearDelay {
				t.Fatalf("GetClearDelay() = %d, want %d after retire", got, DefaultClearDelay)
			}
			if recorder.hasEvent("single-task-runner:updated") {
				t.Fatal("retired service should not emit updated events for rejected work")
			}
			if recorder.hasEvent("single-task-runner:stopped") {
				t.Fatal("retired service should not emit stopped events for rejected work")
			}
		})
	}
}

func TestStartOnRetiredServiceSkipsContextCreation(t *testing.T) {
	t.Parallel()

	newContextCalled := 0
	deps := testDeps(nil, nil)
	deps.NewContext = func() (context.Context, context.CancelFunc) {
		newContextCalled++
		return context.WithCancel(context.Background())
	}
	svc := NewService(deps)
	svc.Retire()

	err := svc.Start()
	if !errors.Is(err, errServiceRetired) {
		t.Fatalf("Start on retired service: error = %v, want %v", err, errServiceRetired)
	}
	if newContextCalled != 0 {
		t.Fatalf("NewContext called %d times, want 0 for retired start", newContextCalled)
	}
}

func TestServiceRetiredStatusSnapshotIsDefault(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	if err := svc.AddItem("Task 1", "message", "%1", false, ""); err != nil {
		t.Fatalf("AddItem(setup): %v", err)
	}
	if err := svc.SetClearDelay(5); err != nil {
		t.Fatalf("SetClearDelay(setup): %v", err)
	}

	svc.Retire()

	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}
	if status.CurrentIndex != -1 {
		t.Fatalf("CurrentIndex = %d, want -1", status.CurrentIndex)
	}
	if len(status.Items) != 0 {
		t.Fatalf("Items len = %d, want 0", len(status.Items))
	}
	if status.SessionName != "" {
		t.Fatalf("SessionName = %q, want empty", status.SessionName)
	}
	if status.GenerationID != "" {
		t.Fatalf("GenerationID = %q, want empty", status.GenerationID)
	}
	if status.ClearDelaySec != DefaultClearDelay {
		t.Fatalf("ClearDelaySec = %d, want %d", status.ClearDelaySec, DefaultClearDelay)
	}
}

func TestStartRejectsCanceledRuntimeContext(t *testing.T) {
	deps := testDeps(nil, nil)
	deps.NewContext = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, func() {}
	}

	svc := NewService(deps)
	if err := svc.AddItem("Task 1", "message", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	err := svc.Start()
	if err == nil || !strings.Contains(err.Error(), "runtime context is unavailable") {
		t.Fatalf("Start() error = %v, want runtime context rejection", err)
	}
}

func TestRunLoopContextCancellationResetsQueueState(t *testing.T) {
	sentCh := make(chan string, 1)
	recorder := &eventRecorder{}
	var (
		runtimeCtx    context.Context
		runtimeCancel context.CancelFunc
	)

	deps := testDeps(sentCh, recorder)
	deps.NewContext = func() (context.Context, context.CancelFunc) {
		runtimeCtx, runtimeCancel = context.WithCancel(context.Background())
		return runtimeCtx, runtimeCancel
	}

	svc := NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh
	updatedBeforeCancel := recorder.countEvents("single-task-runner:updated")

	runtimeCancel()

	waitForCondition(t, time.Second, "queue reset after runtime cancellation", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueIdle &&
			status.CurrentIndex == -1 &&
			len(status.Items) == 1 &&
			status.Items[0].Status == ItemStatusCancelled
	})

	waitForCondition(t, time.Second, "updated event after runtime cancellation", func() bool {
		return recorder.countEvents("single-task-runner:updated") > updatedBeforeCancel
	})
	if runtimeCtx.Err() == nil {
		t.Fatal("runtime context should be cancelled")
	}
}

func TestServiceFailTaskCancelsBeforeStoppedEvent(t *testing.T) {
	sentCh := make(chan string, 1)
	stoppedCancelled := make(chan bool, 1)
	var cancelMu sync.Mutex
	cancelled := false

	deps := testDeps(sentCh, nil)
	deps.NewContext = func() (context.Context, context.CancelFunc) {
		ctx, baseCancel := context.WithCancel(context.Background())
		return ctx, func() {
			cancelMu.Lock()
			cancelled = true
			cancelMu.Unlock()
			baseCancel()
		}
	}
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, _ any) {
		if name != "single-task-runner:stopped" {
			return
		}
		cancelMu.Lock()
		wasCancelled := cancelled
		cancelMu.Unlock()
		stoppedCancelled <- wasCancelled
	})

	svc := NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first instruction", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	<-sentCh
	taskID := svc.GetStatus().Items[0].ID
	if err := svc.FailTask(taskID, "boom"); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	select {
	case wasCancelled := <-stoppedCancelled:
		if !wasCancelled {
			t.Fatal("stopped event was emitted before cancel ran")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stopped event")
	}

	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}
	if status.LastStopReason != "boom" {
		t.Fatalf("LastStopReason = %q, want %q", status.LastStopReason, "boom")
	}
}

func TestHandleWorkerFatalCancelsBeforeStoppedEvent(t *testing.T) {
	stoppedCancelled := make(chan bool, 1)
	var cancelMu sync.Mutex
	cancelled := false

	deps := testDeps(nil, nil)
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, _ any) {
		if name != "single-task-runner:stopped" {
			return
		}
		cancelMu.Lock()
		wasCancelled := cancelled
		cancelMu.Unlock()
		stoppedCancelled <- wasCancelled
	})

	svc := NewService(deps)
	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.currentIndex = 0
	svc.cancel = func() {
		cancelMu.Lock()
		cancelled = true
		cancelMu.Unlock()
	}
	svc.mu.Unlock()

	svc.handleWorkerFatal(svc.generationID, "worker crashed")

	select {
	case wasCancelled := <-stoppedCancelled:
		if !wasCancelled {
			t.Fatal("stopped event was emitted before cancel ran")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stopped event")
	}

	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}
	if status.LastStopReason != "worker crashed" {
		t.Fatalf("LastStopReason = %q, want %q", status.LastStopReason, "worker crashed")
	}
}

func TestServiceFailItemSkipsRuntimeEventsDuringShutdown(t *testing.T) {
	recorder := &eventRecorder{}
	deps := testDeps(nil, recorder)
	deps.IsShuttingDown = func() bool { return true }
	svc := NewService(deps)

	svc.mu.Lock()
	svc.items = []QueueItem{{
		ID:     "task-1",
		Status: ItemStatusActive,
	}}
	svc.runStatus = QueueRunning
	svc.currentIndex = 0
	svc.cancel = func() {}
	svc.mu.Unlock()

	svc.failItem(svc.generationID, 0, "task-1", "boom")

	if recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("stopped event should be suppressed during shutdown")
	}
	if recorder.hasEvent("single-task-runner:updated") {
		t.Fatal("updated event should be suppressed during shutdown")
	}
}

func TestHandleWorkerFatalSkipsRuntimeEventsDuringShutdown(t *testing.T) {
	recorder := &eventRecorder{}
	deps := testDeps(nil, recorder)
	deps.IsShuttingDown = func() bool { return true }
	svc := NewService(deps)

	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.currentIndex = 0
	svc.cancel = func() {}
	svc.mu.Unlock()

	svc.handleWorkerFatal(svc.generationID, "worker crashed")

	if recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("stopped event should be suppressed during shutdown")
	}
	if recorder.hasEvent("single-task-runner:updated") {
		t.Fatal("updated event should be suppressed during shutdown")
	}
}

func TestHandleWorkerFatalClearsCompletionChannels(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.cancel = func() {}
	svc.completionCh["task-1"] = make(chan completionSignal, 1)
	svc.completionCh["task-2"] = make(chan completionSignal, 1)
	svc.mu.Unlock()

	svc.handleWorkerFatal(svc.generationID, "worker crashed")

	svc.completionMu.Lock()
	defer svc.completionMu.Unlock()
	if len(svc.completionCh) != 0 {
		t.Fatalf("completion channel count after worker fatal = %d, want 0", len(svc.completionCh))
	}
}

func TestServiceStartAndCompleteTaskSequence(t *testing.T) {
	sentCh := make(chan string, 4)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first instruction", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.AddItem("Task 2", "second instruction", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	firstMessage := <-sentCh
	firstStatus := svc.GetStatus()
	firstID := firstStatus.Items[0].ID
	secondID := firstStatus.Items[1].ID
	if !strings.Contains(firstMessage, "first instruction") {
		t.Fatalf("first message = %q, want first instruction", firstMessage)
	}
	if !strings.Contains(firstMessage, "[Task Info]") {
		t.Fatalf("first message = %q, want task footer", firstMessage)
	}
	if !strings.Contains(firstMessage, firstID) {
		t.Fatalf("first message = %q, want task id %q", firstMessage, firstID)
	}

	if err := svc.CompleteTask(firstID, "done-1"); err != nil {
		t.Fatalf("CompleteTask first: %v", err)
	}

	secondMessage := <-sentCh
	if !strings.Contains(secondMessage, "second instruction") {
		t.Fatalf("second message = %q, want second instruction", secondMessage)
	}
	if !strings.Contains(secondMessage, secondID) {
		t.Fatalf("second message = %q, want task id %q", secondMessage, secondID)
	}
	if err := svc.CompleteTask(secondID, "done-2"); err != nil {
		t.Fatalf("CompleteTask second: %v", err)
	}

	waitForCondition(t, 2*time.Second, "queue completed with both tasks done", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueCompleted &&
			status.Items[0].Status == ItemStatusDone &&
			status.Items[0].ResultMessage == "done-1" &&
			status.Items[1].Status == ItemStatusDone &&
			status.Items[1].ResultMessage == "done-2"
	})
}

func TestServiceFailTaskStopsQueueAndKeepsPendingItems(t *testing.T) {
	sentCh := make(chan string, 2)
	recorder := &eventRecorder{}
	svc := NewService(testDeps(sentCh, recorder))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first instruction", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.AddItem("Task 2", "second instruction", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	<-sentCh
	firstID := svc.GetStatus().Items[0].ID
	if err := svc.FailTask(firstID, "boom"); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	waitForCondition(t, 2*time.Second, "queue idle with first task failed and second pending", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueIdle &&
			status.CurrentIndex == -1 &&
			status.Items[0].Status == ItemStatusFailed &&
			status.Items[0].ErrorMessage == "boom" &&
			status.Items[1].Status == ItemStatusPending
	})
	if got := svc.GetStatus().LastStopReason; got != "boom" {
		t.Fatalf("LastStopReason = %q, want %q", got, "boom")
	}
	waitForCondition(t, 2*time.Second, "single-task-runner stopped event emitted", func() bool {
		return recorder.hasEvent("single-task-runner:stopped")
	})
}

func TestServiceEnqueueTasksChecksTargetPaneOnce(t *testing.T) {
	var mu sync.Mutex
	checkCalls := 0

	deps := testDeps(nil, nil)
	deps.CheckPaneAlive = func(string) error {
		mu.Lock()
		checkCalls++
		mu.Unlock()
		return nil
	}

	svc := NewService(deps)
	queued, err := svc.EnqueueTasks("%1", []EnqueueTaskInput{
		{Title: "Task 1", Message: "first"},
		{Title: "Task 2", Message: "second"},
	})
	if err != nil {
		t.Fatalf("EnqueueTasks: %v", err)
	}
	if len(queued) != 2 {
		t.Fatalf("queued count = %d, want 2", len(queued))
	}
	if queued[0].OrderIndex != 0 || queued[1].OrderIndex != 1 {
		t.Fatalf("order indexes = [%d, %d], want [0, 1]", queued[0].OrderIndex, queued[1].OrderIndex)
	}

	status := svc.GetStatus()
	if len(status.Items) != 2 {
		t.Fatalf("status items len = %d, want 2", len(status.Items))
	}
	if status.Items[0].OrderIndex != 0 || status.Items[1].OrderIndex != 1 {
		t.Fatalf("item order indexes = [%d, %d], want [0, 1]", status.Items[0].OrderIndex, status.Items[1].OrderIndex)
	}

	mu.Lock()
	gotCalls := checkCalls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("CheckPaneAlive call count = %d, want 1", gotCalls)
	}
}

func TestServiceEnqueueTasksReturnsPaneAvailabilityError(t *testing.T) {
	deps := testDeps(nil, nil)
	deps.CheckPaneAlive = func(string) error {
		return errors.New("pane gone")
	}

	svc := NewService(deps)
	queued, err := svc.EnqueueTasks("%1", []EnqueueTaskInput{
		{Title: "Task 1", Message: "first"},
	})
	if err == nil {
		t.Fatal("EnqueueTasks error = nil, want target pane availability error")
	}
	if queued != nil {
		t.Fatalf("queued = %#v, want nil on pane availability failure", queued)
	}
	if !strings.Contains(err.Error(), "target pane unavailable: pane gone") {
		t.Fatalf("EnqueueTasks error = %q, want wrapped pane availability error", err.Error())
	}
	if got := len(svc.GetStatus().Items); got != 0 {
		t.Fatalf("queue item count = %d, want 0 after pane availability failure", got)
	}
}

func TestServiceEnqueueTasksRollsBackOnValidationError(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	if err := svc.AddItem("Existing", "keep me", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	queued, err := svc.EnqueueTasks("%1", []EnqueueTaskInput{
		{Title: "Task 1", Message: "first"},
		{Title: "", Message: "second"},
	})
	if err == nil {
		t.Fatal("expected validation error from EnqueueTasks")
	}
	if queued != nil {
		t.Fatalf("queued = %#v, want nil on rollback", queued)
	}

	status := svc.GetStatus()
	if len(status.Items) != 1 {
		t.Fatalf("len(status.Items) = %d, want 1", len(status.Items))
	}
	if status.Items[0].Title != "Existing" {
		t.Fatalf("remaining item title = %q, want %q", status.Items[0].Title, "Existing")
	}
}

func TestSignalCompletionLockedDropsDuplicateSignals(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	ch := make(chan completionSignal, 1)
	ch <- completionSignal{finalStatus: ItemStatusDone, message: "first"}

	svc.completionMu.Lock()
	svc.completionCh["task-1"] = ch
	signaled := svc.signalCompletionLocked("task-1", completionSignal{
		finalStatus: ItemStatusFailed,
		message:     "duplicate",
	})
	_, exists := svc.completionCh["task-1"]
	svc.completionMu.Unlock()

	if signaled {
		t.Fatal("signalCompletionLocked should return false when the completion channel is already full")
	}
	if exists {
		t.Fatal("completion channel entry should be deleted after dropping a duplicate signal")
	}

	got := <-ch
	if got.message != "first" {
		t.Fatalf("completion channel message = %q, want original buffered signal", got.message)
	}
}

func TestStartValidation(t *testing.T) {
	t.Run("already running", func(t *testing.T) {
		sentCh := make(chan string, 1)
		svc := NewService(testDeps(sentCh, nil))
		if err := svc.SetClearDelay(0); err != nil {
			t.Fatalf("SetClearDelay: %v", err)
		}
		if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
			t.Fatalf("AddItem: %v", err)
		}
		if err := svc.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		<-sentCh

		err := svc.Start()
		if err == nil || !strings.Contains(err.Error(), "already running") {
			t.Fatalf("Start() error = %v, want already running", err)
		}
	})

	t.Run("no pending tasks", func(t *testing.T) {
		svc := NewService(testDeps(nil, nil))
		err := svc.Start()
		if err == nil || !strings.Contains(err.Error(), "no pending tasks") {
			t.Fatalf("Start() error = %v, want no pending tasks", err)
		}
	})
}

func TestCancelTask(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Service, chan string) string
		cancelErr bool
		validate  func(*testing.T, *Service, string)
	}{
		{
			name: "cancel pending task",
			setup: func(svc *Service, _ chan string) string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				status := svc.GetStatus()
				return status.Items[0].ID
			},
			cancelErr: false,
			validate: func(t *testing.T, svc *Service, taskID string) {
				status := svc.GetStatus()
				item := status.Items[0]
				if item.Status != ItemStatusCancelled {
					t.Errorf("Status = %q, want %q", item.Status, ItemStatusCancelled)
				}
				if item.ErrorMessage != "Cancelled" {
					t.Errorf("ErrorMessage = %q, want 'Cancelled'", item.ErrorMessage)
				}
			},
		},
		{
			name: "cancel active task",
			setup: func(svc *Service, sentCh chan string) string {
				mustSetClearDelay(t, svc, 0)
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				status := svc.GetStatus()
				taskID := status.Items[0].ID
				mustStart(t, svc)
				<-sentCh
				return taskID
			},
			cancelErr: false,
			validate: func(t *testing.T, svc *Service, taskID string) {
				waitForCondition(t, 2*time.Second, "task cancelled", func() bool {
					status := svc.GetStatus()
					return status.Items[0].Status == ItemStatusCancelled
				})
				status := svc.GetStatus()
				if status.Items[0].ErrorMessage != "Cancelled" {
					t.Errorf("ErrorMessage = %q, want 'Cancelled'", status.Items[0].ErrorMessage)
				}
			},
		},
		{
			name: "cancel completed task",
			setup: func(svc *Service, sentCh chan string) string {
				mustSetClearDelay(t, svc, 0)
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				status := svc.GetStatus()
				taskID := status.Items[0].ID
				mustStart(t, svc)
				<-sentCh
				mustCompleteTask(t, svc, taskID, "done")
				waitForCondition(t, 2*time.Second, "task completed", func() bool {
					status := svc.GetStatus()
					return status.Items[0].Status == ItemStatusDone
				})
				return taskID
			},
			cancelErr: true,
			validate: func(t *testing.T, svc *Service, taskID string) {
				status := svc.GetStatus()
				if status.Items[0].Status != ItemStatusDone {
					t.Errorf("Status = %q, want %q", status.Items[0].Status, ItemStatusDone)
				}
			},
		},
		{
			name: "cancel with empty taskID",
			setup: func(_ *Service, _ chan string) string {
				return ""
			},
			cancelErr: true,
			validate: func(t *testing.T, svc *Service, _ string) {
				status := svc.GetStatus()
				if len(status.Items) != 0 {
					t.Errorf("Items count = %d, want 0", len(status.Items))
				}
			},
		},
		{
			name: "cancel nonexistent taskID",
			setup: func(svc *Service, _ chan string) string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				return "t-nonexistent"
			},
			cancelErr: true,
			validate: func(t *testing.T, svc *Service, _ string) {
				status := svc.GetStatus()
				if status.Items[0].Status != ItemStatusPending {
					t.Errorf("Status = %q, want %q", status.Items[0].Status, ItemStatusPending)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentCh := make(chan string, 4)
			svc := NewService(testDeps(sentCh, nil))
			taskID := tt.setup(svc, sentCh)

			err := svc.CancelTask(taskID, "")
			if (err != nil) != tt.cancelErr {
				t.Errorf("CancelTask() error = %v, wantErr %v", err, tt.cancelErr)
			}

			tt.validate(t, svc, taskID)
		})
	}
}

func TestCancelTaskDuringSendingSkipsPasteDelivery(t *testing.T) {
	sentCh := make(chan string, 1)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(1); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "test", "%1", true, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	taskID := svc.GetStatus().Items[0].ID
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForCondition(t, time.Second, "task enters sending state", func() bool {
		status := svc.GetStatus()
		return len(status.Items) == 1 && status.Items[0].Status == ItemStatusSending
	})
	if err := svc.CancelTask(taskID, ""); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	waitForCondition(t, time.Second, "task remains cancelled", func() bool {
		status := svc.GetStatus()
		return len(status.Items) == 1 && status.Items[0].Status == ItemStatusCancelled
	})
	waitForCondition(t, 2*time.Second, "queue completes after cancelling sending task", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueCompleted
	})

	select {
	case sent := <-sentCh:
		t.Fatalf("SendMessagePaste should be skipped for cancelled sending task, got %q", sent)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCancelTaskBeforeCompletionChannelRegistrationDoesNotBlockQueue(t *testing.T) {
	sentCh := make(chan string, 1)
	cancelErrCh := make(chan error, 1)
	var (
		svc        *Service
		cancelOnce sync.Once
	)

	deps := testDeps(sentCh, nil)
	deps.Emitter = apptypes.EventEmitterFunc(func(name string, payload any) {
		if name != "single-task-runner:updated" {
			return
		}
		status, ok := payload.(QueueStatus)
		if !ok || len(status.Items) != 1 || status.Items[0].Status != ItemStatusSending {
			return
		}
		cancelOnce.Do(func() {
			cancelErrCh <- svc.CancelTask(status.Items[0].ID, "")
		})
	})

	svc = NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "test", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case err := <-cancelErrCh:
		if err != nil {
			t.Fatalf("CancelTask: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cancel during sending")
	}

	waitForCondition(t, time.Second, "queue completes after pre-registration cancel", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueCompleted &&
			len(status.Items) == 1 &&
			status.Items[0].Status == ItemStatusCancelled
	})

	select {
	case sent := <-sentCh:
		t.Fatalf("SendMessagePaste should be skipped when cancel wins before channel registration, got %q", sent)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestNormalizeQueueItemInput(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		message    string
		paneID     string
		clearCmd   string
		wantErr    bool
		errPattern string
		validate   func(*testing.T, string, string, string, string)
	}{
		{
			name:     "valid input",
			title:    "Task Title",
			message:  "Task message",
			paneID:   "%1",
			clearCmd: "custom-clear",
			wantErr:  false,
			validate: func(t *testing.T, gotTitle, gotMsg, gotPane, gotCmd string) {
				if gotTitle != "Task Title" {
					t.Errorf("title = %q, want 'Task Title'", gotTitle)
				}
				if gotMsg != "Task message" {
					t.Errorf("message = %q, want 'Task message'", gotMsg)
				}
				if gotPane != "%1" {
					t.Errorf("paneID = %q, want %%1", gotPane)
				}
				if gotCmd != "custom-clear" {
					t.Errorf("clearCmd = %q, want 'custom-clear'", gotCmd)
				}
			},
		},
		{
			name:       "empty title",
			title:      "",
			message:    "message",
			paneID:     "%1",
			clearCmd:   "",
			wantErr:    true,
			errPattern: "title is required",
		},
		{
			name:       "empty message",
			title:      "Title",
			message:    "",
			paneID:     "%1",
			clearCmd:   "",
			wantErr:    true,
			errPattern: "message is required",
		},
		{
			name:       "empty targetPaneID",
			title:      "Title",
			message:    "Message",
			paneID:     "",
			clearCmd:   "",
			wantErr:    true,
			errPattern: "target pane id is required",
		},
		{
			name:       "whitespace-only title",
			title:      "   ",
			message:    "message",
			paneID:     "%1",
			clearCmd:   "",
			wantErr:    true,
			errPattern: "title is required",
		},
		{
			name:       "title exceeds max length",
			title:      strings.Repeat("x", 201),
			message:    "message",
			paneID:     "%1",
			clearCmd:   "",
			wantErr:    true,
			errPattern: "200 characters or fewer",
		},
		{
			name:       "message exceeds max length",
			title:      "Title",
			message:    strings.Repeat("x", 8001),
			paneID:     "%1",
			clearCmd:   "",
			wantErr:    true,
			errPattern: "8000 characters or fewer",
		},
		{
			name:     "clearCommand is trimmed",
			title:    "Title",
			message:  "Message",
			paneID:   "%1",
			clearCmd: "  custom-clear  ",
			wantErr:  false,
			validate: func(t *testing.T, gotTitle, gotMsg, gotPane, gotCmd string) {
				if gotCmd != "custom-clear" {
					t.Errorf("clearCmd = %q, want 'custom-clear' (trimmed)", gotCmd)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotMsg, gotPane, gotCmd, err := normalizeQueueItemInput(tt.title, tt.message, tt.paneID, tt.clearCmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeQueueItemInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.errPattern != "" && !strings.Contains(err.Error(), tt.errPattern) {
					t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errPattern)
				}
				return
			}
			if tt.validate != nil {
				tt.validate(t, gotTitle, gotMsg, gotPane, gotCmd)
			}
		})
	}
}

func TestConcurrentStartStop(t *testing.T) {
	sentCh := make(chan string, 4)
	svc := NewService(testDeps(sentCh, nil))

	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}

	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	<-sentCh

	var stopErr error
	done := make(chan struct{})
	go func() {
		stopErr = svc.Stop()
		close(done)
	}()

	<-done

	if stopErr != nil {
		t.Fatalf("Stop: %v", stopErr)
	}

	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Errorf("RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}
}

func TestExecuteClearPreStep(t *testing.T) {
	tests := []struct {
		name         string
		clearBefore  bool
		clearCommand string
		delaySeconds int
		clearFails   bool
		validate     func(*testing.T, *Service, int)
	}{
		{
			name:        "clear disabled",
			clearBefore: false,
			validate: func(t *testing.T, svc *Service, clearCalls int) {
				if clearCalls != 0 {
					t.Errorf("SendClearCommand calls = %d, want 0", clearCalls)
				}
			},
		},
		{
			name:         "clear with custom command",
			clearBefore:  true,
			clearCommand: "custom-clear",
			delaySeconds: 0,
			validate: func(t *testing.T, svc *Service, clearCalls int) {
				if clearCalls != 1 {
					t.Errorf("SendClearCommand calls = %d, want 1", clearCalls)
				}
			},
		},
		{
			name:         "clear with default command",
			clearBefore:  true,
			clearCommand: "",
			delaySeconds: 0,
			validate: func(t *testing.T, svc *Service, clearCalls int) {
				if clearCalls != 1 {
					t.Errorf("SendClearCommand calls = %d, want 1", clearCalls)
				}
			},
		},
		{
			name:         "SendClearCommand failure marks item failed",
			clearBefore:  true,
			clearFails:   true,
			delaySeconds: 0,
			validate: func(t *testing.T, svc *Service, clearCalls int) {
				status := svc.GetStatus()
				if len(status.Items) == 0 {
					t.Error("no items in queue after clear failure")
					return
				}
				if status.Items[0].Status != ItemStatusFailed {
					t.Errorf("item status = %q, want %q when clear fails", status.Items[0].Status, ItemStatusFailed)
				}
				if !strings.Contains(status.Items[0].ErrorMessage, "clear command: clear failed") {
					t.Errorf("ErrorMessage = %q, want clear-command failure context", status.Items[0].ErrorMessage)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			clearCalls := 0

			deps := testDeps(make(chan string, 4), nil)
			deps.SendClearCommand = func(paneID, cmd string) error {
				mu.Lock()
				clearCalls++
				mu.Unlock()
				if tt.clearFails {
					return errors.New("clear failed")
				}
				return nil
			}

			svc := NewService(deps)
			mustSetClearDelay(t, svc, tt.delaySeconds)
			mustAddItem(t, svc, "Task", "message", "%1", tt.clearBefore, tt.clearCommand)

			mustStart(t, svc)
			waitForCondition(t, time.Second, "clear step observed", func() bool {
				mu.Lock()
				calls := clearCalls
				mu.Unlock()
				if tt.clearFails {
					status := svc.GetStatus()
					return calls == 1 && len(status.Items) == 1 && status.Items[0].Status == ItemStatusFailed
				}
				if tt.clearBefore {
					return calls == 1
				}
				return true
			})
			mu.Lock()
			calls := clearCalls
			mu.Unlock()
			tt.validate(t, svc, calls)
		})
	}
}

func TestExecuteItemSendMessagePasteFailureStopsQueue(t *testing.T) {
	recorder := &eventRecorder{}
	deps := testDeps(nil, recorder)
	deps.SendMessagePaste = func(string, string) error {
		return errors.New("paste failed")
	}

	svc := NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForCondition(t, time.Second, "queue stops after paste failure", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueIdle &&
			status.CurrentIndex == -1 &&
			len(status.Items) == 2 &&
			status.Items[0].Status == ItemStatusFailed &&
			status.Items[1].Status == ItemStatusPending
	})

	status := svc.GetStatus()
	if !strings.Contains(status.Items[0].ErrorMessage, "send message: paste failed") {
		t.Fatalf("ErrorMessage = %q, want send-message failure context", status.Items[0].ErrorMessage)
	}
	if !recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("stopped event should be emitted after paste failure")
	}
}

func TestSetClearDelayValidation(t *testing.T) {
	tests := []struct {
		name      string
		delaySec  int
		wantErr   bool
		wantValue int
	}{
		{name: "minimum", delaySec: MinClearDelaySec, wantValue: MinClearDelaySec},
		{name: "maximum", delaySec: MaxClearDelaySec, wantValue: MaxClearDelaySec},
		{name: "negative", delaySec: -1, wantErr: true, wantValue: DefaultClearDelay},
		{name: "above maximum", delaySec: MaxClearDelaySec + 1, wantErr: true, wantValue: DefaultClearDelay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			err := svc.SetClearDelay(tt.delaySec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetClearDelay(%d) error = %v, wantErr %v", tt.delaySec, err, tt.wantErr)
			}
			if got := svc.GetClearDelay(); got != tt.wantValue {
				t.Fatalf("GetClearDelay() = %d, want %d", got, tt.wantValue)
			}
		})
	}
}

func TestRemoveItem(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Service) string
		removeErr bool
		validate  func(*testing.T, *Service)
	}{
		{
			name: "remove pending item",
			setup: func(svc *Service) string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				mustAddItem(t, svc, "Task 2", "test", "%1", false, "")
				status := svc.GetStatus()
				return status.Items[0].ID
			},
			removeErr: false,
			validate: func(t *testing.T, svc *Service) {
				status := svc.GetStatus()
				if len(status.Items) != 1 {
					t.Errorf("Items count = %d, want 1", len(status.Items))
				}
				if status.Items[0].OrderIndex != 0 {
					t.Errorf("OrderIndex = %d, want 0", status.Items[0].OrderIndex)
				}
			},
		},
		{
			name: "remove with empty id",
			setup: func(svc *Service) string {
				return ""
			},
			removeErr: true,
		},
		{
			name: "remove nonexistent item",
			setup: func(svc *Service) string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				return "t-nonexistent"
			},
			removeErr: true,
			validate: func(t *testing.T, svc *Service) {
				status := svc.GetStatus()
				if len(status.Items) != 1 {
					t.Errorf("Items count = %d, want 1 (unchanged)", len(status.Items))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			itemID := tt.setup(svc)
			err := svc.RemoveItem(itemID)
			if (err != nil) != tt.removeErr {
				t.Errorf("RemoveItem() error = %v, wantErr %v", err, tt.removeErr)
			}
			if tt.validate != nil {
				tt.validate(t, svc)
			}
		})
	}
}

func TestRemoveItemRejectsActiveItem(t *testing.T) {
	sentCh := make(chan string, 1)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	taskID := svc.GetStatus().Items[0].ID
	err := svc.RemoveItem(taskID)
	if err == nil || !strings.Contains(err.Error(), "cannot remove item") {
		t.Fatalf("RemoveItem() error = %v, want active item rejection", err)
	}
}

func TestReorderItems(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*Service) []string
		orderedIDs []string
		wantErr    bool
		validate   func(*testing.T, *Service)
	}{
		{
			name: "reorder items successfully",
			setup: func(svc *Service) []string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				mustAddItem(t, svc, "Task 2", "test", "%1", false, "")
				status := svc.GetStatus()
				return []string{status.Items[0].ID, status.Items[1].ID}
			},
			orderedIDs: nil,
			wantErr:    false,
			validate: func(t *testing.T, svc *Service) {
				status := svc.GetStatus()
				if status.Items[0].Title != "Task 2" || status.Items[1].Title != "Task 1" {
					t.Errorf("Items titles = [%q, %q], want ['Task 2', 'Task 1']", status.Items[0].Title, status.Items[1].Title)
				}
			},
		},
		{
			name:       "reorder with empty list",
			orderedIDs: []string{},
			wantErr:    true,
		},
		{
			name: "reorder with mismatched count",
			setup: func(svc *Service) []string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				mustAddItem(t, svc, "Task 2", "test", "%1", false, "")
				return nil
			},
			orderedIDs: []string{"id1"},
			wantErr:    true,
		},
		{
			name: "reorder with duplicate ids",
			setup: func(svc *Service) []string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				mustAddItem(t, svc, "Task 2", "test", "%1", false, "")
				status := svc.GetStatus()
				return []string{status.Items[0].ID, status.Items[1].ID}
			},
			orderedIDs: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			orderedIDs := tt.orderedIDs
			if tt.setup != nil {
				ids := tt.setup(svc)
				if ids != nil && orderedIDs == nil && tt.name == "reorder items successfully" {
					orderedIDs = []string{ids[1], ids[0]}
				}
				if ids != nil && orderedIDs == nil && tt.name == "reorder with duplicate ids" {
					orderedIDs = []string{ids[0], ids[0]}
				}
			}
			err := svc.ReorderItems(orderedIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReorderItems() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, svc)
			}
		})
	}
}

func TestReorderItemsRejectsRunningQueue(t *testing.T) {
	sentCh := make(chan string, 1)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}
	status := svc.GetStatus()
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	err := svc.ReorderItems([]string{status.Items[1].ID, status.Items[0].ID})
	if err == nil || !strings.Contains(err.Error(), "cannot reorder while queue is running") {
		t.Fatalf("ReorderItems() error = %v, want running queue rejection", err)
	}
}

func TestUpdateItem(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Service) string
		newTitle string
		newMsg   string
		newPane  string
		wantErr  bool
		validate func(*testing.T, *Service)
	}{
		{
			name: "update pending item",
			setup: func(svc *Service) string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				status := svc.GetStatus()
				return status.Items[0].ID
			},
			newTitle: "New Title",
			newMsg:   "New Message",
			newPane:  "%1",
			wantErr:  false,
			validate: func(t *testing.T, svc *Service) {
				status := svc.GetStatus()
				if status.Items[0].Title != "New Title" {
					t.Errorf("Title = %q, want 'New Title'", status.Items[0].Title)
				}
				if status.Items[0].Message != "New Message" {
					t.Errorf("Message = %q, want 'New Message'", status.Items[0].Message)
				}
			},
		},
		{
			name: "update with empty item id",
			setup: func(_ *Service) string {
				return ""
			},
			wantErr: true,
		},
		{
			name: "update nonexistent item",
			setup: func(svc *Service) string {
				mustAddItem(t, svc, "Task 1", "test", "%1", false, "")
				return "t-nonexistent"
			},
			newTitle: "New Title",
			newMsg:   "New Message",
			newPane:  "%1",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			itemID := tt.setup(svc)
			err := svc.UpdateItem(itemID, tt.newTitle, tt.newMsg, tt.newPane, false, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateItem() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, svc)
			}
		})
	}
}

func TestUpdateItemRejectsActiveItem(t *testing.T) {
	sentCh := make(chan string, 1)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	taskID := svc.GetStatus().Items[0].ID
	err := svc.UpdateItem(taskID, "Updated", "updated", "%1", false, "")
	if err == nil || !strings.Contains(err.Error(), "cannot update item") {
		t.Fatalf("UpdateItem() error = %v, want active item rejection", err)
	}
}

func TestUpdateItemResetsTerminalStateToPending(t *testing.T) {
	tests := []struct {
		name         string
		prepareState func(*Service, string)
	}{
		{
			name: "done item resets",
			prepareState: func(svc *Service, taskID string) {
				svc.mu.Lock()
				svc.items[0].Status = ItemStatusDone
				svc.items[0].StartedAt = "2026-01-01T00:00:00Z"
				svc.items[0].CompletedAt = "2026-01-01T00:01:00Z"
				svc.items[0].ResultMessage = "done"
				svc.items[0].ErrorMessage = ""
				svc.mu.Unlock()
			},
		},
		{
			name: "failed item resets",
			prepareState: func(svc *Service, taskID string) {
				svc.mu.Lock()
				svc.items[0].Status = ItemStatusFailed
				svc.items[0].StartedAt = "2026-01-01T00:00:00Z"
				svc.items[0].CompletedAt = "2026-01-01T00:01:00Z"
				svc.items[0].ErrorMessage = "failed"
				svc.items[0].ResultMessage = ""
				svc.mu.Unlock()
			},
		},
		{
			name: "cancelled item resets",
			prepareState: func(svc *Service, taskID string) {
				svc.mu.Lock()
				svc.items[0].Status = ItemStatusCancelled
				svc.items[0].StartedAt = "2026-01-01T00:00:00Z"
				svc.items[0].CompletedAt = "2026-01-01T00:01:00Z"
				svc.items[0].ErrorMessage = "stop"
				svc.items[0].ResultMessage = ""
				svc.mu.Unlock()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			if err := svc.AddItem("Task 1", "test", "%1", false, ""); err != nil {
				t.Fatalf("AddItem: %v", err)
			}
			taskID := svc.GetStatus().Items[0].ID
			tt.prepareState(svc, taskID)

			if err := svc.UpdateItem(taskID, "Updated", "updated message", "%2", true, "/clear"); err != nil {
				t.Fatalf("UpdateItem: %v", err)
			}

			item := svc.GetStatus().Items[0]
			if item.Status != ItemStatusPending {
				t.Fatalf("Status = %q, want %q", item.Status, ItemStatusPending)
			}
			if runStatus := svc.GetStatus().RunStatus; runStatus != QueueIdle {
				t.Fatalf("RunStatus = %q, want %q", runStatus, QueueIdle)
			}
			if item.StartedAt != "" {
				t.Fatalf("StartedAt = %q, want empty", item.StartedAt)
			}
			if item.CompletedAt != "" {
				t.Fatalf("CompletedAt = %q, want empty", item.CompletedAt)
			}
			if item.ErrorMessage != "" {
				t.Fatalf("ErrorMessage = %q, want empty", item.ErrorMessage)
			}
			if item.ResultMessage != "" {
				t.Fatalf("ResultMessage = %q, want empty", item.ResultMessage)
			}
			if item.TargetPaneID != "%2" {
				t.Fatalf("TargetPaneID = %q, want %q", item.TargetPaneID, "%2")
			}
			if !item.ClearBefore {
				t.Fatal("ClearBefore = false, want true")
			}
			if item.ClearCommand != "/clear" {
				t.Fatalf("ClearCommand = %q, want %q", item.ClearCommand, "/clear")
			}
		})
	}
}

func TestCompleteAndFailTaskRejectNonActiveStatus(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Service) string
		action func(*Service, string) error
	}{
		{
			name: "complete pending task",
			setup: func(svc *Service) string {
				if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
					t.Fatalf("AddItem: %v", err)
				}
				return svc.GetStatus().Items[0].ID
			},
			action: func(svc *Service, taskID string) error {
				return svc.CompleteTask(taskID, "done")
			},
		},
		{
			name: "fail cancelled task",
			setup: func(svc *Service) string {
				if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
					t.Fatalf("AddItem: %v", err)
				}
				taskID := svc.GetStatus().Items[0].ID
				if err := svc.CancelTask(taskID, "stop"); err != nil {
					t.Fatalf("CancelTask: %v", err)
				}
				return taskID
			},
			action: func(svc *Service, taskID string) error {
				return svc.FailTask(taskID, "failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			taskID := tt.setup(svc)
			err := tt.action(svc, taskID)
			if err == nil || !strings.Contains(err.Error(), "is not active") {
				t.Fatalf("action error = %v, want non-active rejection", err)
			}
		})
	}
}

func TestStopAllIsNoopWhenQueueIsIdle(t *testing.T) {
	recorder := &eventRecorder{}
	svc := NewService(testDeps(nil, recorder))
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	svc.StopAll()

	status := svc.GetStatus()
	if status.RunStatus != QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}
	if recorder.countEvents("single-task-runner:updated") != 1 {
		t.Fatalf("updated event count = %d, want 1", recorder.countEvents("single-task-runner:updated"))
	}
	if recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("unexpected stopped event for idle StopAll")
	}
}

func TestStopEmitsStoppedEventWhenQueueIsRunning(t *testing.T) {
	recorder := &eventRecorder{}
	svc := NewService(testDeps(nil, recorder))

	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.cancel = func() {}
	svc.items = []QueueItem{{ID: "task-1", Status: ItemStatusActive}}
	svc.mu.Unlock()

	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if !recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("stopped event should be emitted for Stop")
	}
}

func TestStopAllEmitsStoppedEventWhenQueueIsRunning(t *testing.T) {
	recorder := &eventRecorder{}
	svc := NewService(testDeps(nil, recorder))

	svc.mu.Lock()
	svc.runStatus = QueueRunning
	svc.cancel = func() {}
	svc.items = []QueueItem{{ID: "task-1", Status: ItemStatusActive}}
	svc.mu.Unlock()

	svc.StopAll()

	if !recorder.hasEvent("single-task-runner:stopped") {
		t.Fatal("stopped event should be emitted for StopAll")
	}
}

func TestCompleteAndFailTaskRejectTooLongMessages(t *testing.T) {
	tests := []struct {
		name    string
		action  func(*Service, string) error
		wantErr string
	}{
		{
			name: "complete result too long",
			action: func(svc *Service, taskID string) error {
				return svc.CompleteTask(taskID, strings.Repeat("r", maxTaskResultLen+1))
			},
			wantErr: "result must be",
		},
		{
			name: "fail reason too long",
			action: func(svc *Service, taskID string) error {
				return svc.FailTask(taskID, strings.Repeat("f", maxTaskFailureReason+1))
			},
			wantErr: "reason must be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(testDeps(nil, nil))
			err := tt.action(svc, "task-1")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuildTaskMessageUsesPlaceholderTaskIDWhenMissing(t *testing.T) {
	message := buildTaskMessage("", "Do the thing")
	if !strings.Contains(message, "task_id: <task_id>") {
		t.Fatalf("message = %q, want placeholder task id", message)
	}
	if !strings.Contains(message, "If the task should not continue: call cancel_task with this task_id.") {
		t.Fatalf("message = %q, want cancel_task guidance", message)
	}
}

func TestQueueItemStatusMethods(t *testing.T) {
	tests := []struct {
		status   QueueItemStatus
		editable bool
		terminal bool
	}{
		{ItemStatusPending, true, false},
		{ItemStatusSending, false, false},
		{ItemStatusActive, false, false},
		{ItemStatusDone, true, true},
		{ItemStatusFailed, true, true},
		{ItemStatusCancelled, true, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status)+"_IsEditable", func(t *testing.T) {
			got := tt.status.IsEditable()
			if got != tt.editable {
				t.Errorf("QueueItemStatus(%q).IsEditable() = %v, want %v", tt.status, got, tt.editable)
			}
		})
		t.Run(string(tt.status)+"_IsTerminal", func(t *testing.T) {
			got := tt.status.IsTerminal()
			if got != tt.terminal {
				t.Errorf("QueueItemStatus(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}

func TestCancelTaskStatusChangedWhileWaitingForSendMu(t *testing.T) {
	pasteStarted := make(chan struct{})
	releasePaste := make(chan struct{})
	errCh := make(chan error, 1)

	deps := testDeps(nil, nil)
	deps.SendMessagePaste = func(string, string) error {
		close(pasteStarted)
		<-releasePaste
		return nil
	}

	svc := NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	taskID := svc.GetStatus().Items[0].ID
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	<-pasteStarted
	go func() {
		errCh <- svc.CancelTask(taskID, "")
	}()

	svc.StopAll()
	close(releasePaste)

	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "cannot cancel task") || !strings.Contains(err.Error(), string(ItemStatusCancelled)) {
		t.Fatalf("CancelTask() error = %v, want cancelled-status rejection", err)
	}
}

func TestCancelTaskDuringClearDelayAdvancesQueueImmediately(t *testing.T) {
	sentCh := make(chan string, 2)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(5); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", true, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}
	status := svc.GetStatus()
	firstID := status.Items[0].ID
	secondID := status.Items[1].ID
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForCondition(t, time.Second, "first task enters clear-delay sending state", func() bool {
		current := svc.GetStatus()
		return current.RunStatus == QueueRunning && current.Items[0].Status == ItemStatusSending
	})

	started := time.Now()
	if err := svc.CancelTask(firstID, ""); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	select {
	case message := <-sentCh:
		if !strings.Contains(message, "second") {
			t.Fatalf("sent message = %q, want second task payload", message)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("timed out waiting for the next task after cancelling clear delay")
	}

	if elapsed := time.Since(started); elapsed >= 1500*time.Millisecond {
		t.Fatalf("queue advanced after %v, want faster than clear delay", elapsed)
	}
	if err := svc.CompleteTask(secondID, "done"); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	waitForCondition(t, 2*time.Second, "queue completes after second task", func() bool {
		current := svc.GetStatus()
		return current.RunStatus == QueueCompleted &&
			current.Items[0].Status == ItemStatusCancelled &&
			current.Items[1].Status == ItemStatusDone
	})
}

func TestClearDelayInterruptedByContextCancellation(t *testing.T) {
	var runtimeCancel context.CancelFunc

	deps := testDeps(nil, nil)
	deps.NewContext = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		runtimeCancel = cancel
		return ctx, cancel
	}

	svc := NewService(deps)
	if err := svc.SetClearDelay(10); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", true, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForCondition(t, time.Second, "task enters clear-delay sending state", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueRunning && status.Items[0].Status == ItemStatusSending
	})

	started := time.Now()
	runtimeCancel()

	waitForCondition(t, time.Second, "queue resets after cancelling clear delay", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueIdle &&
			status.CurrentIndex == -1 &&
			len(status.Items) == 1 &&
			status.Items[0].Status == ItemStatusCancelled
	})
	if elapsed := time.Since(started); elapsed >= 5*time.Second {
		t.Fatalf("clear delay cancellation took %v, want less than 5s", elapsed)
	}
	if got := svc.GetStatus().LastStopReason; got != "runLoop terminated" {
		t.Fatalf("LastStopReason = %q, want %q", got, "runLoop terminated")
	}
}

func TestStartAfterCompletedQueueRotatesGenerationForNewWork(t *testing.T) {
	sentCh := make(chan string, 2)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start(first): %v", err)
	}
	<-sentCh
	firstID := svc.GetStatus().Items[0].ID
	if err := svc.CompleteTask(firstID, "done"); err != nil {
		t.Fatalf("CompleteTask(first): %v", err)
	}
	waitForCondition(t, 2*time.Second, "first run completed", func() bool {
		return svc.GetStatus().RunStatus == QueueCompleted
	})
	completedGeneration := svc.GetStatus().GenerationID

	if err := svc.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}
	if got := svc.GetStatus().RunStatus; got != QueueIdle {
		t.Fatalf("RunStatus after enqueueing new work = %q, want %q", got, QueueIdle)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start(second): %v", err)
	}

	secondStatus := svc.GetStatus()
	if secondStatus.GenerationID == completedGeneration {
		t.Fatal("expected Start to rotate generation_id after a completed queue")
	}
}

func TestStopDoesNotDoubleCancelWhenRunLoopExits(t *testing.T) {
	sentCh := make(chan string, 1)
	var (
		cancelMu    sync.Mutex
		cancelCalls int
	)

	deps := testDeps(sentCh, nil)
	deps.NewContext = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, func() {
			cancelMu.Lock()
			cancelCalls++
			cancelMu.Unlock()
			cancel()
		}
	}

	svc := NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitForCondition(t, time.Second, "queue stops cleanly", func() bool {
		return svc.GetStatus().RunStatus == QueueIdle
	})

	cancelMu.Lock()
	got := cancelCalls
	cancelMu.Unlock()
	if got != 1 {
		t.Fatalf("cancel call count = %d, want 1", got)
	}
}

func TestExecuteItemCheckPaneAliveFailureStopsQueue(t *testing.T) {
	reorderCalls := 0
	deps := testDeps(nil, nil)
	deps.CheckPaneAlive = func(string) error {
		reorderCalls++
		if reorderCalls == 1 {
			return nil
		}
		return errors.New("pane gone")
	}

	svc := NewService(deps)
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForCondition(t, time.Second, "queue stops after pane check failure", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueIdle &&
			len(status.Items) == 1 &&
			status.Items[0].Status == ItemStatusFailed
	})

	status := svc.GetStatus()
	if !strings.Contains(status.Items[0].ErrorMessage, "target pane unavailable: pane gone") {
		t.Fatalf("ErrorMessage = %q, want pane-unavailable context", status.Items[0].ErrorMessage)
	}
	if status.LastStopReason != "target pane unavailable: pane gone" {
		t.Fatalf("LastStopReason = %q, want pane-unavailable context", status.LastStopReason)
	}
}

func TestStopAllIsIdempotent(t *testing.T) {
	sentCh := make(chan string, 1)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	svc.StopAll()
	svc.StopAll()

	waitForCondition(t, time.Second, "StopAll remains idle after repeated calls", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueIdle &&
			len(status.Items) == 1 &&
			status.Items[0].Status == ItemStatusCancelled
	})
}

func TestCompleteTaskAndCancelTaskConcurrent(t *testing.T) {
	sentCh := make(chan string, 1)
	svc := NewService(testDeps(sentCh, nil))
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	taskID := svc.GetStatus().Items[0].ID
	start := make(chan struct{})
	errCh := make(chan error, 2)

	go func() {
		<-start
		errCh <- svc.CompleteTask(taskID, "done")
	}()
	go func() {
		<-start
		errCh <- svc.CancelTask(taskID, "stop")
	}()
	close(start)

	for range 2 {
		if err := <-errCh; !isAllowedConcurrentTaskResolutionError(err) {
			t.Fatalf("concurrent resolution error = %v, want nil or expected race outcome", err)
		}
	}

	waitForCondition(t, 2*time.Second, "task reaches a terminal state", func() bool {
		status := svc.GetStatus()
		return status.RunStatus == QueueCompleted &&
			len(status.Items) == 1 &&
			status.Items[0].Status.IsTerminal()
	})
	status := svc.GetStatus()
	if status.Items[0].Status == ItemStatusSending || status.Items[0].Status == ItemStatusActive {
		t.Fatalf("Status = %q, want a terminal state", status.Items[0].Status)
	}
}

func TestEnqueueTasksRejectsEmptyTargetPaneID(t *testing.T) {
	svc := NewService(testDeps(nil, nil))
	queued, err := svc.EnqueueTasks("", []EnqueueTaskInput{{Title: "Task 1", Message: "first"}})
	if err == nil || !strings.Contains(err.Error(), "target pane id is required") {
		t.Fatalf("EnqueueTasks() error = %v, want empty-target rejection", err)
	}
	if queued != nil {
		t.Fatalf("queued = %#v, want nil on validation failure", queued)
	}
}

func isAllowedConcurrentTaskResolutionError(err error) bool {
	if err == nil {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "not awaiting completion") ||
		strings.Contains(message, "is not active") ||
		strings.Contains(message, "cannot cancel task")
}
