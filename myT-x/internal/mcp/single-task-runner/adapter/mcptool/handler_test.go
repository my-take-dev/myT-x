package mcptool

import (
	"context"
	"strings"
	"testing"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/singletaskrunner"
	"myT-x/internal/workerutil"
)

func waitForHandlerCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func testHandlerDeps(sentCh chan<- string) singletaskrunner.Deps {
	return singletaskrunner.Deps{
		Emitter:          apptypes.NoopEmitter{},
		IsShuttingDown:   func() bool { return false },
		CheckPaneAlive:   func(string) error { return nil },
		SendClearCommand: func(string, string) error { return nil },
		SendMessagePaste: func(_ string, message string) error {
			if sentCh != nil {
				sentCh <- message
			}
			return nil
		},
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

func extractListItems(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()

	items, ok := payload["items"].([]map[string]any)
	if ok {
		return items
	}

	rawItems, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("items type = %T, want []map[string]any or []any", payload["items"])
	}

	items = make([]map[string]any, 0, len(rawItems))
	for i, item := range rawItems {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("items[%d] type = %T, want map[string]any", i, item)
		}
		items = append(items, entry)
	}
	return items
}

func TestHandleEnqueueTaskAndListQueue(t *testing.T) {
	t.Parallel()

	service := singletaskrunner.NewService(testHandlerDeps(nil))
	handler := NewHandler(service)

	result, err := handler.handleEnqueueTask(context.Background(), map[string]any{
		"target_pane": "%1",
		"tasks": []any{
			map[string]any{"message": "first"},
			map[string]any{"title": "Second", "message": "second"},
		},
	})
	if err != nil {
		t.Fatalf("handleEnqueueTask: %v", err)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["queue_length"] != 2 {
		t.Fatalf("queue_length = %v, want 2", payload["queue_length"])
	}

	listResult, err := handler.handleListQueue(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handleListQueue: %v", err)
	}
	listPayload, ok := listResult.(map[string]any)
	if !ok {
		t.Fatalf("list result type = %T, want map[string]any", listResult)
	}
	items := extractListItems(t, listPayload)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0]["title"] != "first" {
		t.Fatalf("items[0].title = %v, want %q", items[0]["title"], "first")
	}
	if items[0]["status"] != singletaskrunner.ItemStatusPending {
		t.Fatalf("items[0].status = %v, want %q", items[0]["status"], singletaskrunner.ItemStatusPending)
	}
	if items[1]["title"] != "Second" {
		t.Fatalf("items[1].title = %v, want %q", items[1]["title"], "Second")
	}
	if items[1]["message"] != "second" {
		t.Fatalf("items[1].message = %v, want %q", items[1]["message"], "second")
	}
}

func TestHandleEnqueueTaskValidatesRequiredInputs(t *testing.T) {
	t.Parallel()

	service := singletaskrunner.NewService(testHandlerDeps(nil))
	handler := NewHandler(service)

	if _, err := handler.handleEnqueueTask(context.Background(), map[string]any{
		"tasks": []any{map[string]any{"message": "first"}},
	}); err == nil {
		t.Fatal("expected target_pane validation error")
	}

	if _, err := handler.handleEnqueueTask(context.Background(), map[string]any{
		"target_pane": "%1",
	}); err == nil {
		t.Fatal("expected tasks validation error")
	}
}

func TestHandleCompleteTaskReturnsNextTaskID(t *testing.T) {
	sentCh := make(chan string, 4)
	service := singletaskrunner.NewService(testHandlerDeps(sentCh))
	if err := service.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := service.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := service.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}
	if err := service.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	handler := NewHandler(service)
	status := service.GetStatus()
	firstID := status.Items[0].ID
	secondID := status.Items[1].ID

	result, err := handler.handleCompleteTask(context.Background(), map[string]any{
		"task_id": firstID,
		"result":  "ok",
	})
	if err != nil {
		t.Fatalf("handleCompleteTask: %v", err)
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["next_task_id"] != secondID {
		t.Fatalf("next_task_id = %v, want %s", payload["next_task_id"], secondID)
	}

	secondMessage := <-sentCh
	if !strings.Contains(secondMessage, secondID) {
		t.Fatalf("second message = %q, want task id %q", secondMessage, secondID)
	}
	if err := service.CompleteTask(secondID, "done"); err != nil {
		t.Fatalf("CompleteTask second: %v", err)
	}
	waitForHandlerCondition(t, 2*time.Second, func() bool {
		return service.GetStatus().RunStatus == singletaskrunner.QueueCompleted
	})
}

func TestHandleFailTaskReturnsRemainingTaskCount(t *testing.T) {
	sentCh := make(chan string, 4)
	service := singletaskrunner.NewService(testHandlerDeps(sentCh))
	if err := service.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := service.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 1: %v", err)
	}
	if err := service.AddItem("Task 2", "second", "%1", false, ""); err != nil {
		t.Fatalf("AddItem 2: %v", err)
	}
	if err := service.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-sentCh

	handler := NewHandler(service)
	firstID := service.GetStatus().Items[0].ID
	result, err := handler.handleFailTask(context.Background(), map[string]any{
		"task_id": firstID,
		"reason":  "boom",
	})
	if err != nil {
		t.Fatalf("handleFailTask: %v", err)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["remaining_tasks"] != 1 {
		t.Fatalf("remaining_tasks = %v, want 1", payload["remaining_tasks"])
	}
}

func TestHandleCancelTaskReturnsCancelledStatus(t *testing.T) {
	service := singletaskrunner.NewService(testHandlerDeps(nil))
	if err := service.AddItem("Task 1", "first", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	handler := NewHandler(service)
	taskID := service.GetStatus().Items[0].ID
	result, err := handler.handleCancelTask(context.Background(), map[string]any{
		"task_id": taskID,
		"reason":  "stop",
	})
	if err != nil {
		t.Fatalf("handleCancelTask: %v", err)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["status"] != "cancelled" {
		t.Fatalf("status = %v, want cancelled", payload["status"])
	}
	if service.GetStatus().Items[0].Status != singletaskrunner.ItemStatusCancelled {
		t.Fatalf("item status = %q, want %q", service.GetStatus().Items[0].Status, singletaskrunner.ItemStatusCancelled)
	}
}

func TestHandleCancelTaskRequiresTaskID(t *testing.T) {
	t.Parallel()

	service := singletaskrunner.NewService(testHandlerDeps(nil))
	handler := NewHandler(service)

	if _, err := handler.handleCancelTask(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected task_id validation error")
	}
}

func TestHandleHelpOverviewMentionsCancelTask(t *testing.T) {
	t.Parallel()

	result, err := handleHelp(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handleHelp: %v", err)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	overview, _ := payload["overview"].(string)
	if !strings.Contains(overview, "cancel_task") {
		t.Fatalf("overview = %q, want cancel_task guidance", overview)
	}

	workflow, ok := payload["workflow"].([]string)
	if !ok {
		t.Fatalf("workflow type = %T, want []string", payload["workflow"])
	}
	if len(workflow) != 3 || !strings.Contains(workflow[2], "cancel_task") {
		t.Fatalf("workflow = %#v, want cancel_task guidance in step 3", workflow)
	}
}

func TestBuildRegistryRequiresService(t *testing.T) {
	handler := NewHandler(nil)
	if _, err := handler.BuildRegistry(); err == nil {
		t.Fatal("expected BuildRegistry to fail without a service")
	}
}
