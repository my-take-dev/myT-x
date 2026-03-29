package main

import (
	"strings"
	"testing"
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/scheduler"
	"myT-x/internal/tmux"
)

// ------------------------------------------------------------
// App-level scheduler integration tests.
// These verify that the Wails-bound facade methods correctly delegate
// to the scheduler.Service via the dependency injection wiring in NewApp.
// Detailed behavior is tested in internal/scheduler/service_test.go.
// ------------------------------------------------------------

func TestStartSchedulerValidation(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, err := app.StartScheduler("", "%1", "hello", 1, 1)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("unexpected error: %v", err)
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

func TestDeleteSchedulerNotFound(t *testing.T) {
	app := NewApp()
	err := app.DeleteScheduler("non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent scheduler")
	}
}

func TestStopAllSchedulersEmpty(t *testing.T) {
	app := NewApp()
	err := app.StopAllSchedulers()
	if err != nil {
		t.Fatalf("StopAllSchedulers() error = %v", err)
	}
}

func TestGetSchedulerStatusesEmpty(t *testing.T) {
	app := NewApp()
	statuses := app.GetSchedulerStatuses()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses, got %d", len(statuses))
	}
}

// TestStartAndStopSchedulerIntegration verifies the full lifecycle through the App facade.
func TestStartAndStopSchedulerIntegration(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	id, err := app.StartScheduler("test", pane.IDString(), "hello", 60, 1)
	if err != nil {
		t.Fatalf("StartScheduler() error = %v", err)
	}

	statuses := app.GetSchedulerStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Running {
		t.Fatal("entry should be running")
	}

	if err := app.StopScheduler(id); err != nil {
		t.Fatalf("StopScheduler() error = %v", err)
	}

	statuses = app.GetSchedulerStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status after stop, got %d", len(statuses))
	}
	if statuses[0].Running {
		t.Fatal("entry should be stopped")
	}

	if err := app.DeleteScheduler(id); err != nil {
		t.Fatalf("DeleteScheduler() error = %v", err)
	}

	statuses = app.GetSchedulerStatuses()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses after delete, got %d", len(statuses))
	}
}

// TestResumeSchedulerIntegration verifies resume through the App facade.
func TestResumeSchedulerIntegration(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	id, err := app.StartScheduler("resume", pane.IDString(), "hello", 60, 2)
	if err != nil {
		t.Fatalf("StartScheduler() error = %v", err)
	}

	if err := app.StopScheduler(id); err != nil {
		t.Fatalf("StopScheduler() error = %v", err)
	}

	if err := app.ResumeScheduler(id); err != nil {
		t.Fatalf("ResumeScheduler() error = %v", err)
	}

	statuses := app.GetSchedulerStatuses()
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

// TestStopAllSchedulersIntegration verifies bulk stop through the App facade.
func TestStopAllSchedulersIntegration(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	_, pane, err := app.sessions.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	for _, title := range []string{"a", "b", "c"} {
		if _, err := app.StartScheduler(title, pane.IDString(), "hello", 60, 1); err != nil {
			t.Fatalf("StartScheduler(%q) error = %v", title, err)
		}
	}

	if err := app.StopAllSchedulers(); err != nil {
		t.Fatalf("StopAllSchedulers() error = %v", err)
	}

	statuses := app.GetSchedulerStatuses()
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s.Running {
			t.Fatalf("entry %s should be stopped", s.ID)
		}
	}
}

// ------------------------------------------------------------
// isPaneAlive tests (shared utility, stays in app package)
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
// schedulerSendMessage tests (shared utility, stays in app package)
// ------------------------------------------------------------

func TestSchedulerSendMessage(t *testing.T) {
	t.Run("success with select-pane", func(t *testing.T) {
		var calls []string
		sk := callRecorder(&calls)

		err := sk.schedulerSendMessage(nil, "%1", "hello world")
		if err != nil {
			t.Fatalf("schedulerSendMessage() error = %v", err)
		}
		// Expect select-pane first, then send-keys with message and Enter.
		if len(calls) < 2 {
			t.Fatalf("got %d calls, want at least 2: %v", len(calls), calls)
		}
		if calls[0] != "select-pane:%1" {
			t.Errorf("calls[0] = %q, want %q", calls[0], "select-pane:%1")
		}
	})

	t.Run("trims trailing newlines", func(t *testing.T) {
		var captured ipc.TmuxRequest
		sk := sendKeysIO{
			executeRequest: func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
				captured = req
				return ipc.TmuxResponse{ExitCode: 0}
			},
			sleep: func(time.Duration) {},
		}

		err := sk.schedulerSendMessage(nil, "%1", "hello\n\r")
		if err != nil {
			t.Fatalf("schedulerSendMessage() error = %v", err)
		}
		// The last captured request is send-keys (after select-pane).
		if len(captured.Args) != 2 || captured.Args[0] != "hello" || captured.Args[1] != "Enter" {
			t.Errorf("Args = %v, want [hello Enter]", captured.Args)
		}
	})

	t.Run("select-pane failure returns error", func(t *testing.T) {
		sk := failOnCommand("select-pane")

		err := sk.schedulerSendMessage(nil, "%1", "hello")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "select-pane failed") {
			t.Errorf("error = %q, want containing %q", err.Error(), "select-pane failed")
		}
	})

	t.Run("send-keys failure returns error", func(t *testing.T) {
		sk := failOnCommand("send-keys")

		err := sk.schedulerSendMessage(nil, "%1", "hello")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "send-keys failed") {
			t.Errorf("error = %q, want containing %q", err.Error(), "send-keys failed")
		}
	})
}

// ------------------------------------------------------------
// Template persistence integration tests (via App facade)
// ------------------------------------------------------------

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

func TestSaveAndLoadSchedulerTemplate(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	tmpl := scheduler.Template{
		Title: "Deploy Check", Message: "check deploy status",
		IntervalSeconds: 30, MaxCount: 10,
	}
	if err := app.SaveSchedulerTemplate("test-session", tmpl); err != nil {
		t.Fatalf("SaveSchedulerTemplate() error = %v", err)
	}

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
}

func TestDeleteSchedulerTemplateNonExistent(t *testing.T) {
	app, _ := setupTemplateTestApp(t)
	err := app.DeleteSchedulerTemplate("test-session", "ghost")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
}

func TestDeleteSchedulerTemplate(t *testing.T) {
	app, _ := setupTemplateTestApp(t)

	for _, title := range []string{"Keep", "Remove"} {
		if err := app.SaveSchedulerTemplate("test-session", scheduler.Template{
			Title: title, Message: "msg", IntervalSeconds: 10, MaxCount: 1,
		}); err != nil {
			t.Fatalf("save %q error = %v", title, err)
		}
	}

	if err := app.DeleteSchedulerTemplate("test-session", "Remove"); err != nil {
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
