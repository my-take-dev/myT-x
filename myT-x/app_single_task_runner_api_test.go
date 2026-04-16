package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	"myT-x/internal/ipc"
	"myT-x/internal/singletaskrunner"
	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"
)

func mustSessionKey(t *testing.T, app *App, sessionName string) string {
	t.Helper()

	snapshot, err := app.sessionService.FindSessionSnapshotByName(sessionName)
	if err != nil {
		t.Fatalf("FindSessionSnapshotByName(%q): %v", sessionName, err)
	}
	return buildSessionKey(snapshot.Name, snapshot.ID)
}

func TestGetSingleTaskRunnerStatusWithoutActiveSession(t *testing.T) {
	app := NewApp()

	status, err := app.GetSingleTaskRunnerStatus("")
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() error = %v, want nil", err)
	}
	if status.RunStatus != singletaskrunner.QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, singletaskrunner.QueueIdle)
	}
	if status.CurrentIndex != -1 {
		t.Fatalf("CurrentIndex = %d, want -1", status.CurrentIndex)
	}
	if len(status.Items) != 0 {
		t.Fatalf("Items len = %d, want 0", len(status.Items))
	}
	if status.ClearDelaySec != singletaskrunner.DefaultClearDelay {
		t.Fatalf("ClearDelaySec = %d, want %d", status.ClearDelaySec, singletaskrunner.DefaultClearDelay)
	}
}

func TestGetSingleTaskRunnerClearDelayWithoutActiveSession(t *testing.T) {
	app := NewApp()

	delay, err := app.GetSingleTaskRunnerClearDelay("")
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerClearDelay() error = %v, want nil", err)
	}
	if delay != singletaskrunner.DefaultClearDelay {
		t.Fatalf("GetSingleTaskRunnerClearDelay() = %d, want %d", delay, singletaskrunner.DefaultClearDelay)
	}
}

func TestSingleTaskRunnerAPIsRequireActiveSession(t *testing.T) {
	app := NewApp()

	tests := []struct {
		name string
		call func() error
	}{
		{name: "start", call: func() error { return app.StartSingleTaskRunner("") }},
		{name: "stop", call: func() error { return app.StopSingleTaskRunner("") }},
		{name: "add item", call: func() error { return app.AddSingleTaskRunnerItem("", "Task", "message", "%1", false, "") }},
		{name: "remove item", call: func() error { return app.RemoveSingleTaskRunnerItem("", "task-1") }},
		{name: "update item", call: func() error { return app.UpdateSingleTaskRunnerItem("", "task-1", "Task", "message", "%1", false, "") }},
		{name: "reorder items", call: func() error { return app.ReorderSingleTaskRunnerItems("", []string{"task-1"}) }},
		{name: "set clear delay", call: func() error { return app.SetSingleTaskRunnerClearDelay("", 3) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected error for missing active session")
			}
			if !strings.Contains(err.Error(), "no active session") {
				t.Fatalf("error = %v, want no active session", err)
			}
		})
	}
}

func TestSingleTaskRunnerAPIsRequireSessionKeyWhenActiveSessionExists(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.SetActiveSession("session-a")

	tests := []struct {
		name string
		call func() error
	}{
		{name: "get status", call: func() error { _, err := app.GetSingleTaskRunnerStatus(""); return err }},
		{name: "get clear delay", call: func() error { _, err := app.GetSingleTaskRunnerClearDelay(""); return err }},
		{name: "start", call: func() error { return app.StartSingleTaskRunner("") }},
		{name: "stop", call: func() error { return app.StopSingleTaskRunner("") }},
		{name: "add item", call: func() error { return app.AddSingleTaskRunnerItem("", "Task", "message", "%1", false, "") }},
		{name: "remove item", call: func() error { return app.RemoveSingleTaskRunnerItem("", "task-1") }},
		{name: "update item", call: func() error { return app.UpdateSingleTaskRunnerItem("", "task-1", "Task", "message", "%1", false, "") }},
		{name: "reorder items", call: func() error { return app.ReorderSingleTaskRunnerItems("", []string{"task-1"}) }},
		{name: "set clear delay", call: func() error { return app.SetSingleTaskRunnerClearDelay("", 3) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected error for missing session key")
			}
			if !strings.Contains(err.Error(), errSessionKeyRequired.Error()) {
				t.Fatalf("error = %v, want %q", err, errSessionKeyRequired)
			}
		})
	}
}

func TestSingleTaskRunnerAPIsRejectMismatchedSessionKey(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	}
	if _, _, err := app.sessions.CreateSession("session-b", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-b): %v", err)
	}
	app.SetActiveSession("session-a")
	staleKey := mustSessionKey(t, app, "session-b")

	tests := []struct {
		name string
		call func() error
	}{
		{name: "get status", call: func() error { _, err := app.GetSingleTaskRunnerStatus(staleKey); return err }},
		{name: "get clear delay", call: func() error { _, err := app.GetSingleTaskRunnerClearDelay(staleKey); return err }},
		{name: "start", call: func() error { return app.StartSingleTaskRunner(staleKey) }},
		{name: "stop", call: func() error { return app.StopSingleTaskRunner(staleKey) }},
		{name: "add item", call: func() error { return app.AddSingleTaskRunnerItem(staleKey, "Task", "message", "%1", false, "") }},
		{name: "remove item", call: func() error { return app.RemoveSingleTaskRunnerItem(staleKey, "task-1") }},
		{name: "update item", call: func() error {
			return app.UpdateSingleTaskRunnerItem(staleKey, "task-1", "Task", "message", "%1", false, "")
		}},
		{name: "reorder items", call: func() error { return app.ReorderSingleTaskRunnerItems(staleKey, []string{"task-1"}) }},
		{name: "set clear delay", call: func() error { return app.SetSingleTaskRunnerClearDelay(staleKey, 3) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected error for mismatched session key")
			}
			if !strings.Contains(err.Error(), "session key mismatch") {
				t.Fatalf("error = %v, want session key mismatch", err)
			}
		})
	}
}

func TestSingleTaskRunnerReadOnlyAPIsRejectBareSessionName(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.SetActiveSession("session-a")

	tests := []struct {
		name string
		call func() error
	}{
		{name: "get status", call: func() error { _, err := app.GetSingleTaskRunnerStatus("session-a"); return err }},
		{name: "get clear delay", call: func() error { _, err := app.GetSingleTaskRunnerClearDelay("session-a"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected error for bare session name")
			}
			if !strings.Contains(err.Error(), "session key mismatch") {
				t.Fatalf("error = %v, want session key mismatch", err)
			}
		})
	}
}

func TestSingleTaskRunnerAPIsUseRouterRenamedActiveSession(t *testing.T) {
	app := NewApp()
	stubRuntimeEventsEmit(t)
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)
	app.router = tmux.NewCommandRouter(app.sessions, apptypes.NoopEmitter{}, app.newRouterOptions(config.DefaultConfig()))

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.SetActiveSession("session-a")

	resp := app.router.Execute(ipc.TmuxRequest{
		Command: "rename-session",
		Flags:   map[string]any{"-t": "session-a"},
		Args:    []string{"session-renamed"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("rename-session exit code = %d, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	sessionKey := mustSessionKey(t, app, "session-renamed")
	status, err := app.GetSingleTaskRunnerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() after router rename error = %v", err)
	}
	if status.SessionName != "session-renamed" {
		t.Fatalf("SessionName = %q, want %q", status.SessionName, "session-renamed")
	}
	if got := app.sessionService.GetActiveSessionName(); got != "session-renamed" {
		t.Fatalf("GetActiveSessionName() = %q, want %q", got, "session-renamed")
	}
}

func TestSingleTaskRunnerAPIsClearRouterDestroyedActiveSession(t *testing.T) {
	app := NewApp()
	stubRuntimeEventsEmit(t)
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)
	app.router = tmux.NewCommandRouter(app.sessions, apptypes.NoopEmitter{}, app.newRouterOptions(config.DefaultConfig()))

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.SetActiveSession("session-a")

	resp := app.router.Execute(ipc.TmuxRequest{
		Command: "kill-session",
		Flags:   map[string]any{"-t": "session-a"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("kill-session exit code = %d, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	status, err := app.GetSingleTaskRunnerStatus("")
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() after router destroy error = %v", err)
	}
	if status.RunStatus != singletaskrunner.QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, singletaskrunner.QueueIdle)
	}
	if got := app.sessionService.GetActiveSessionName(); got != "" {
		t.Fatalf("GetActiveSessionName() = %q, want empty after destroy", got)
	}
}

func TestSingleTaskRunnerAPIsClearKilledActiveSession(t *testing.T) {
	app := NewApp()
	stubRuntimeEventsEmit(t)
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)
	app.router = tmux.NewCommandRouter(app.sessions, apptypes.NoopEmitter{}, app.newRouterOptions(config.DefaultConfig()))

	if _, pane, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	} else {
		app.SetActiveSession("session-a")
		sessionKey := mustSessionKey(t, app, "session-a")
		if err := app.AddSingleTaskRunnerItem(sessionKey, "Task", "message", pane.IDString(), false, ""); err != nil {
			t.Fatalf("AddSingleTaskRunnerItem: %v", err)
		}
	}

	if err := app.KillSession("session-a", false); err != nil {
		t.Fatalf("KillSession: %v", err)
	}

	status, err := app.GetSingleTaskRunnerStatus("")
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() after KillSession error = %v", err)
	}
	if status.RunStatus != singletaskrunner.QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, singletaskrunner.QueueIdle)
	}
	if len(status.Items) != 0 {
		t.Fatalf("Items len = %d, want 0 after KillSession", len(status.Items))
	}
	if got := app.sessionService.GetActiveSessionName(); got != "" {
		t.Fatalf("GetActiveSessionName() = %q, want empty after KillSession", got)
	}
}

func TestSingleTaskRunnerAPIRejectsTargetPaneFromDifferentSession(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	}
	if _, pane, err := app.sessions.CreateSession("session-b", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-b): %v", err)
	} else {
		app.SetActiveSession("session-a")
		sessionKey := mustSessionKey(t, app, "session-a")

		err = app.AddSingleTaskRunnerItem(sessionKey, "Task", "message", pane.IDString(), false, "")
		if err == nil {
			t.Fatal("AddSingleTaskRunnerItem() error = nil, want cross-session pane validation failure")
		}
		if !strings.Contains(err.Error(), "belongs to session session-b, not session-a") {
			t.Fatalf("error = %q, want cross-session pane validation context", err.Error())
		}
	}
}

func TestSingleTaskRunnerAPIUsesValidatedSessionService(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, pane, err := app.sessions.CreateSession("session-a", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.SetActiveSession("session-a")
	sessionKey := mustSessionKey(t, app, "session-a")

	gotDelay, err := app.GetSingleTaskRunnerClearDelay(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerClearDelay() error = %v", err)
	}
	if gotDelay != singletaskrunner.DefaultClearDelay {
		t.Fatalf("GetSingleTaskRunnerClearDelay() = %d, want %d", gotDelay, singletaskrunner.DefaultClearDelay)
	}

	if err := app.AddSingleTaskRunnerItem(sessionKey, "Task", "message", pane.IDString(), false, ""); err != nil {
		t.Fatalf("AddSingleTaskRunnerItem: %v", err)
	}

	status, err := app.GetSingleTaskRunnerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() error = %v", err)
	}
	if status.SessionName != "session-a" {
		t.Fatalf("SessionName = %q, want %q", status.SessionName, "session-a")
	}
	if len(status.Items) != 1 {
		t.Fatalf("Items len = %d, want 1", len(status.Items))
	}

	if err := app.SetSingleTaskRunnerClearDelay(sessionKey, 4); err != nil {
		t.Fatalf("SetSingleTaskRunnerClearDelay: %v", err)
	}
	gotDelay, err = app.GetSingleTaskRunnerClearDelay(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerClearDelay() error = %v", err)
	}
	if gotDelay != 4 {
		t.Fatalf("GetSingleTaskRunnerClearDelay() = %d, want 4", gotDelay)
	}

	if err := app.RemoveSingleTaskRunnerItem(sessionKey, status.Items[0].ID); err != nil {
		t.Fatalf("RemoveSingleTaskRunnerItem: %v", err)
	}

	status, err = app.GetSingleTaskRunnerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() error = %v", err)
	}
	if len(status.Items) != 0 {
		t.Fatalf("Items len after remove = %d, want 0", len(status.Items))
	}
}

func TestSingleTaskRunnerAPIMutationMethodsUseValidatedSessionService(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.singleTaskRunnerManager = singletaskrunner.NewServiceManager(func(sessionName string) singletaskrunner.Deps {
		return singletaskrunner.Deps{
			CheckPaneAlive:   func(string) error { return nil },
			SendMessagePaste: func(string, string) error { return nil },
			SendClearCommand: func(string, string) error { return nil },
			NewContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			LaunchWorker: func(_ string, ctx context.Context, fn func(context.Context), _ workerutil.RecoveryOptions) {
				go fn(ctx)
			},
			BaseRecoveryOptions: func() workerutil.RecoveryOptions {
				return workerutil.RecoveryOptions{MaxRetries: 0}
			},
			SessionName: sessionName,
		}
	})
	app.SetActiveSession("session-a")
	sessionKey := mustSessionKey(t, app, "session-a")

	if err := app.AddSingleTaskRunnerItem(sessionKey, "Task 1", "message-1", "%1", false, ""); err != nil {
		t.Fatalf("AddSingleTaskRunnerItem 1: %v", err)
	}
	if err := app.AddSingleTaskRunnerItem(sessionKey, "Task 2", "message-2", "%1", false, ""); err != nil {
		t.Fatalf("AddSingleTaskRunnerItem 2: %v", err)
	}

	status, err := app.GetSingleTaskRunnerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() error = %v", err)
	}
	if len(status.Items) != 2 {
		t.Fatalf("Items len = %d, want 2", len(status.Items))
	}
	firstID := status.Items[0].ID
	secondID := status.Items[1].ID

	if err := app.UpdateSingleTaskRunnerItem(sessionKey, firstID, "Updated Task", "updated message", "%1", true, "clear"); err != nil {
		t.Fatalf("UpdateSingleTaskRunnerItem: %v", err)
	}
	if err := app.ReorderSingleTaskRunnerItems(sessionKey, []string{secondID, firstID}); err != nil {
		t.Fatalf("ReorderSingleTaskRunnerItems: %v", err)
	}

	status, err = app.GetSingleTaskRunnerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() error = %v", err)
	}
	if status.Items[0].ID != secondID {
		t.Fatalf("first queued item = %q, want %q", status.Items[0].ID, secondID)
	}
	if status.Items[1].Title != "Updated Task" {
		t.Fatalf("updated title = %q, want %q", status.Items[1].Title, "Updated Task")
	}
	if !status.Items[1].ClearBefore {
		t.Fatal("updated item ClearBefore = false, want true")
	}
	if status.Items[1].ClearCommand != "clear" {
		t.Fatalf("updated clear command = %q, want %q", status.Items[1].ClearCommand, "clear")
	}

	if err := app.StartSingleTaskRunner(sessionKey); err != nil {
		t.Fatalf("StartSingleTaskRunner: %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		current, err := app.GetSingleTaskRunnerStatus(sessionKey)
		return err == nil &&
			current.RunStatus == singletaskrunner.QueueRunning &&
			current.CurrentIndex == 0 &&
			len(current.Items) == 2 &&
			current.Items[0].ID == secondID
	}, "single task runner should start with the reordered first item")

	if err := app.StopSingleTaskRunner(sessionKey); err != nil {
		t.Fatalf("StopSingleTaskRunner: %v", err)
	}

	status, err = app.GetSingleTaskRunnerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetSingleTaskRunnerStatus() error = %v", err)
	}
	if status.RunStatus != singletaskrunner.QueueIdle {
		t.Fatalf("RunStatus after stop = %q, want %q", status.RunStatus, singletaskrunner.QueueIdle)
	}
	if status.CurrentIndex != -1 {
		t.Fatalf("CurrentIndex after stop = %d, want -1", status.CurrentIndex)
	}
}
