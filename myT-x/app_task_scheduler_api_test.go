package main

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	"myT-x/internal/ipc"
	"myT-x/internal/taskscheduler"
	"myT-x/internal/tmux"
)

func TestCheckTaskSchedulerOrchestratorReadyWithoutActiveSession(t *testing.T) {
	app := NewApp()

	readiness, err := app.CheckTaskSchedulerOrchestratorReady("")
	if err != nil {
		t.Fatalf("CheckTaskSchedulerOrchestratorReady() error = %v, want nil", err)
	}
	if readiness.Ready {
		t.Fatal("Readiness.Ready = true, want false without an active session")
	}
	if readiness.DBExists {
		t.Fatal("Readiness.DBExists = true, want false without an active session")
	}
	if readiness.AgentCount != 0 {
		t.Fatalf("Readiness.AgentCount = %d, want 0", readiness.AgentCount)
	}
	if readiness.HasPanes {
		t.Fatal("Readiness.HasPanes = true, want false without an active session")
	}
}

func TestGetTaskSchedulerStatusWithoutActiveSessionUsesDefaultConfig(t *testing.T) {
	app := NewApp()

	status, err := app.GetTaskSchedulerStatus("")
	if err != nil {
		t.Fatalf("GetTaskSchedulerStatus() error = %v, want nil", err)
	}
	if status.RunStatus != taskscheduler.QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, taskscheduler.QueueIdle)
	}
	if status.Config.PreExecResetDelay != defaultPreExecResetDelay {
		t.Fatalf("PreExecResetDelay = %d, want %d", status.Config.PreExecResetDelay, defaultPreExecResetDelay)
	}
	if status.Config.PreExecIdleTimeout != defaultPreExecIdleTimeout {
		t.Fatalf("PreExecIdleTimeout = %d, want %d", status.Config.PreExecIdleTimeout, defaultPreExecIdleTimeout)
	}
	if status.Config.PreExecTargetMode != taskscheduler.PreExecTargetMode(defaultPreExecTargetMode) {
		t.Fatalf("PreExecTargetMode = %q, want %q", status.Config.PreExecTargetMode, defaultPreExecTargetMode)
	}
}

func TestTaskSchedulerQueueStatusFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 7
	if got := reflect.TypeFor[TaskSchedulerQueueStatus]().NumField(); got != expectedFieldCount {
		t.Fatalf("TaskSchedulerQueueStatus field count = %d, want %d; update defaultTaskSchedulerQueueStatus and frontend bindings", got, expectedFieldCount)
	}
}

func TestTaskSchedulerAPIsRequireActiveSession(t *testing.T) {
	app := NewApp()
	config := TaskSchedulerQueueConfig{}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "start", call: func() error { return app.StartTaskScheduler("", config, nil) }},
		{name: "stop", call: func() error { return app.StopTaskScheduler("") }},
		{name: "pause", call: func() error { return app.PauseTaskScheduler("") }},
		{name: "resume", call: func() error { return app.ResumeTaskScheduler("") }},
		{name: "add item", call: func() error { return app.AddTaskSchedulerItem("", "Task", "message", "%1", false, "") }},
		{name: "remove item", call: func() error { return app.RemoveTaskSchedulerItem("", "task-1") }},
		{name: "reorder items", call: func() error { return app.ReorderTaskSchedulerItems("", []string{"task-1"}) }},
		{name: "update item", call: func() error { return app.UpdateTaskSchedulerItem("", "task-1", "Task", "message", "%1", false, "") }},
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

func TestTaskSchedulerAPIsRequireSessionKeyWhenActiveSessionExists(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	config := TaskSchedulerQueueConfig{}

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	app.SetActiveSession("session-a")

	tests := []struct {
		name string
		call func() error
	}{
		{name: "get status", call: func() error { _, err := app.GetTaskSchedulerStatus(""); return err }},
		{name: "check readiness", call: func() error { _, err := app.CheckTaskSchedulerOrchestratorReady(""); return err }},
		{name: "start", call: func() error { return app.StartTaskScheduler("", config, nil) }},
		{name: "stop", call: func() error { return app.StopTaskScheduler("") }},
		{name: "pause", call: func() error { return app.PauseTaskScheduler("") }},
		{name: "resume", call: func() error { return app.ResumeTaskScheduler("") }},
		{name: "add item", call: func() error { return app.AddTaskSchedulerItem("", "Task", "message", "%1", false, "") }},
		{name: "remove item", call: func() error { return app.RemoveTaskSchedulerItem("", "task-1") }},
		{name: "reorder items", call: func() error { return app.ReorderTaskSchedulerItems("", []string{"task-1"}) }},
		{name: "update item", call: func() error { return app.UpdateTaskSchedulerItem("", "task-1", "Task", "message", "%1", false, "") }},
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

func TestTaskSchedulerAPIsRejectMismatchedSessionKey(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	config := TaskSchedulerQueueConfig{}

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
		{name: "get status", call: func() error { _, err := app.GetTaskSchedulerStatus(staleKey); return err }},
		{name: "check readiness", call: func() error { _, err := app.CheckTaskSchedulerOrchestratorReady(staleKey); return err }},
		{name: "start", call: func() error { return app.StartTaskScheduler(staleKey, config, nil) }},
		{name: "stop", call: func() error { return app.StopTaskScheduler(staleKey) }},
		{name: "pause", call: func() error { return app.PauseTaskScheduler(staleKey) }},
		{name: "resume", call: func() error { return app.ResumeTaskScheduler(staleKey) }},
		{name: "add item", call: func() error { return app.AddTaskSchedulerItem(staleKey, "Task", "message", "%1", false, "") }},
		{name: "remove item", call: func() error { return app.RemoveTaskSchedulerItem(staleKey, "task-1") }},
		{name: "reorder items", call: func() error { return app.ReorderTaskSchedulerItems(staleKey, []string{"task-1"}) }},
		{name: "update item", call: func() error {
			return app.UpdateTaskSchedulerItem(staleKey, "task-1", "Task", "message", "%1", false, "")
		}},
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

func TestTaskSchedulerReadOnlyAPIsRejectBareSessionName(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	}
	app.SetActiveSession("session-a")

	tests := []struct {
		name string
		call func() error
	}{
		{name: "get status", call: func() error { _, err := app.GetTaskSchedulerStatus("session-a"); return err }},
		{name: "check readiness", call: func() error { _, err := app.CheckTaskSchedulerOrchestratorReady("session-a"); return err }},
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

func TestTaskSchedulerAPIsUseRouterRenamedActiveSession(t *testing.T) {
	app := NewApp()
	stubRuntimeEventsEmit(t)
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)
	app.router = tmux.NewCommandRouter(app.sessions, apptypes.NoopEmitter{}, app.newRouterOptions(config.DefaultConfig()))

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
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
	status, err := app.GetTaskSchedulerStatus(sessionKey)
	if err != nil {
		t.Fatalf("GetTaskSchedulerStatus() after router rename error = %v", err)
	}
	if status.SessionName != "session-renamed" {
		t.Fatalf("SessionName = %q, want %q", status.SessionName, "session-renamed")
	}
	if got := app.sessionService.GetActiveSessionName(); got != "session-renamed" {
		t.Fatalf("GetActiveSessionName() = %q, want %q", got, "session-renamed")
	}
}

func TestTaskSchedulerAPIsClearRouterDestroyedActiveSession(t *testing.T) {
	app := NewApp()
	stubRuntimeEventsEmit(t)
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)
	app.router = tmux.NewCommandRouter(app.sessions, apptypes.NoopEmitter{}, app.newRouterOptions(config.DefaultConfig()))

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	}
	app.SetActiveSession("session-a")

	resp := app.router.Execute(ipc.TmuxRequest{
		Command: "kill-session",
		Flags:   map[string]any{"-t": "session-a"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("kill-session exit code = %d, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	status, err := app.GetTaskSchedulerStatus("")
	if err != nil {
		t.Fatalf("GetTaskSchedulerStatus() after router destroy error = %v", err)
	}
	if status.RunStatus != taskscheduler.QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, taskscheduler.QueueIdle)
	}
	if got := app.sessionService.GetActiveSessionName(); got != "" {
		t.Fatalf("GetActiveSessionName() = %q, want empty after destroy", got)
	}
}

func TestTaskSchedulerAPIsClearKilledActiveSession(t *testing.T) {
	app := NewApp()
	stubRuntimeEventsEmit(t)
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)
	app.router = tmux.NewCommandRouter(app.sessions, apptypes.NoopEmitter{}, app.newRouterOptions(config.DefaultConfig()))

	if _, pane, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	} else {
		app.SetActiveSession("session-a")
		sessionKey := mustSessionKey(t, app, "session-a")
		if err := app.AddTaskSchedulerItem(sessionKey, "Task", "message", pane.IDString(), false, ""); err != nil {
			t.Fatalf("AddTaskSchedulerItem: %v", err)
		}
	}

	if err := app.KillSession("session-a", false); err != nil {
		t.Fatalf("KillSession: %v", err)
	}

	status, err := app.GetTaskSchedulerStatus("")
	if err != nil {
		t.Fatalf("GetTaskSchedulerStatus() after KillSession error = %v", err)
	}
	if status.RunStatus != taskscheduler.QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, taskscheduler.QueueIdle)
	}
	if len(status.Items) != 0 {
		t.Fatalf("Items len = %d, want 0 after KillSession", len(status.Items))
	}
	if got := app.sessionService.GetActiveSessionName(); got != "" {
		t.Fatalf("GetActiveSessionName() = %q, want empty after KillSession", got)
	}
}

func TestTaskSchedulerAPIRejectsTargetPaneFromDifferentSession(t *testing.T) {
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

		err = app.AddTaskSchedulerItem(sessionKey, "Task", "message", pane.IDString(), false, "")
		if err == nil {
			t.Fatal("AddTaskSchedulerItem() error = nil, want cross-session pane validation failure")
		}
		if !strings.Contains(err.Error(), "belongs to session session-b, not session-a") {
			t.Fatalf("error = %q, want cross-session pane validation context", err.Error())
		}
	}
}

func TestTaskSchedulerAPIRejectsUpdatedTargetPaneFromDifferentSession(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, paneA, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	} else if _, paneB, err := app.sessions.CreateSession("session-b", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(session-b): %v", err)
	} else {
		app.SetActiveSession("session-a")
		sessionKey := mustSessionKey(t, app, "session-a")
		if err := app.AddTaskSchedulerItem(sessionKey, "Task", "message", paneA.IDString(), false, ""); err != nil {
			t.Fatalf("AddTaskSchedulerItem: %v", err)
		}

		status, err := app.GetTaskSchedulerStatus(sessionKey)
		if err != nil {
			t.Fatalf("GetTaskSchedulerStatus: %v", err)
		}
		if len(status.Items) != 1 {
			t.Fatalf("Items len = %d, want 1", len(status.Items))
		}

		err = app.UpdateTaskSchedulerItem(sessionKey, status.Items[0].ID, "Task", "message", paneB.IDString(), false, "")
		if err == nil {
			t.Fatal("UpdateTaskSchedulerItem() error = nil, want cross-session pane validation failure")
		}
		if !strings.Contains(err.Error(), "belongs to session session-b, not session-a") {
			t.Fatalf("error = %q, want cross-session pane validation context", err.Error())
		}
	}
}

func TestCheckTaskSchedulerOrchestratorReadyReturnsErrorWhenSourceRootIsUnavailable(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.SetActiveSession("session-a")
	sessionKey := mustSessionKey(t, app, "session-a")

	_, err := app.CheckTaskSchedulerOrchestratorReady(sessionKey)
	if err == nil {
		t.Fatal("CheckTaskSchedulerOrchestratorReady() error = nil, want source-root failure")
	}
	if !strings.Contains(err.Error(), "resolve task scheduler source root") {
		t.Fatalf("error = %q, want source-root failure context", err.Error())
	}
}

func TestCheckTaskSchedulerOrchestratorReadyReturnsReadinessWhenSourceRootExists(t *testing.T) {
	app := NewApp()
	app.configState.Initialize(filepath.Join(t.TempDir(), "config.yaml"), config.DefaultConfig())
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	rootDir := t.TempDir()
	if err := app.sessions.SetRootPath("session-a", rootDir); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}
	app.SetActiveSession("session-a")
	sessionKey := mustSessionKey(t, app, "session-a")

	readiness, err := app.CheckTaskSchedulerOrchestratorReady(sessionKey)
	if err != nil {
		t.Fatalf("CheckTaskSchedulerOrchestratorReady() error = %v", err)
	}
	if readiness.DBExists {
		t.Fatal("Readiness.DBExists = true, want false when orchestrator.db is missing")
	}
	if readiness.AgentCount != 0 {
		t.Fatalf("Readiness.AgentCount = %d, want 0", readiness.AgentCount)
	}
	if !readiness.HasPanes {
		t.Fatal("Readiness.HasPanes = false, want true for a session with panes")
	}
	if readiness.Ready {
		t.Fatal("Readiness.Ready = true, want false without orchestrator.db")
	}
}

func TestCheckTaskSchedulerOrchestratorReadyUsesSessionInfoDBPath(t *testing.T) {
	app := NewApp()
	app.configState.Initialize(filepath.Join(t.TempDir(), "config.yaml"), config.DefaultConfig())
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	db, rootDir := createOrchestratorTaskTestDB(t, app)
	defer db.Close()
	if err := app.sessions.SetRootPath("session-a", rootDir); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO agents (name, pane_id, role, mcp_instance_id) VALUES (?, ?, ?, ?)",
		"agent1",
		"%1",
		"developer",
		"test-instance-1",
	); err != nil {
		t.Fatalf("insert agent error: %v", err)
	}
	app.SetActiveSession("session-a")
	sessionKey := mustSessionKey(t, app, "session-a")

	readiness, err := app.CheckTaskSchedulerOrchestratorReady(sessionKey)
	if err != nil {
		t.Fatalf("CheckTaskSchedulerOrchestratorReady() error = %v", err)
	}
	if !readiness.DBExists {
		t.Fatal("Readiness.DBExists = false, want true")
	}
	if readiness.AgentCount != 1 {
		t.Fatalf("Readiness.AgentCount = %d, want 1", readiness.AgentCount)
	}
	if !readiness.Ready {
		t.Fatal("Readiness.Ready = false, want true")
	}
}

func TestTaskSchedulerAPIUsesValidatedSessionService(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.taskSchedulerManager = newLifecycleTaskSchedulerManager()

	if _, pane, err := app.sessions.CreateSession("session-a", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession: %v", err)
	} else {
		app.SetActiveSession("session-a")
		sessionKey := mustSessionKey(t, app, "session-a")

		if err := app.AddTaskSchedulerItem(sessionKey, "Task 1", "message-1", pane.IDString(), false, ""); err != nil {
			t.Fatalf("AddTaskSchedulerItem 1: %v", err)
		}
		if err := app.AddTaskSchedulerItem(sessionKey, "Task 2", "message-2", pane.IDString(), false, ""); err != nil {
			t.Fatalf("AddTaskSchedulerItem 2: %v", err)
		}

		status, err := app.GetTaskSchedulerStatus(sessionKey)
		if err != nil {
			t.Fatalf("GetTaskSchedulerStatus() error = %v", err)
		}
		if status.SessionName != "session-a" {
			t.Fatalf("SessionName = %q, want %q", status.SessionName, "session-a")
		}
		if len(status.Items) != 2 {
			t.Fatalf("Items len = %d, want 2", len(status.Items))
		}

		firstID := status.Items[0].ID
		secondID := status.Items[1].ID
		if err := app.UpdateTaskSchedulerItem(sessionKey, firstID, "Updated Task", "updated message", pane.IDString(), true, "clear"); err != nil {
			t.Fatalf("UpdateTaskSchedulerItem: %v", err)
		}
		if err := app.ReorderTaskSchedulerItems(sessionKey, []string{secondID, firstID}); err != nil {
			t.Fatalf("ReorderTaskSchedulerItems: %v", err)
		}
		if err := app.RemoveTaskSchedulerItem(sessionKey, secondID); err != nil {
			t.Fatalf("RemoveTaskSchedulerItem: %v", err)
		}

		status, err = app.GetTaskSchedulerStatus(sessionKey)
		if err != nil {
			t.Fatalf("GetTaskSchedulerStatus() error = %v", err)
		}
		if len(status.Items) != 1 {
			t.Fatalf("Items len after remove = %d, want 1", len(status.Items))
		}
		if status.Items[0].Title != "Updated Task" {
			t.Fatalf("updated title = %q, want %q", status.Items[0].Title, "Updated Task")
		}
		if status.Items[0].Status != taskscheduler.ItemStatusPending {
			t.Fatalf("updated status = %q, want %q", status.Items[0].Status, taskscheduler.ItemStatusPending)
		}
	}
}
