package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	"myT-x/internal/ipc"
	"myT-x/internal/session"
	"myT-x/internal/tmux"
)

// newConfigPathForTest sets up a temporary LOCALAPPDATA directory and returns
// a config file path under the standard config directory for the given fileName.
// This is the shared version of the per-file helpers (replaces duplicates in
// app_config_api_test.go and app_task_scheduler_settings_api_test.go).
func newConfigPathForTest(t *testing.T, fileName string) string {
	t.Helper()
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")
	defaultPath := config.DefaultPath()
	return filepath.Join(filepath.Dir(defaultPath), fileName)
}

// newConfigStateForTest creates a StateService initialized with the given
// config path and default config. This is a shared test helper that replaces
// the removed App.configPath field in struct literals.
func newConfigStateForTest(configPath string) *config.StateService {
	s := config.NewStateService()
	s.Initialize(configPath, config.DefaultConfig())
	return s
}

// stubRuntimeEventsEmit replaces runtimeEventsEmitFn with a no-op for the
// duration of the test and restores the original via t.Cleanup.
// Do NOT call t.Parallel() in tests that use this helper;
// package-level variable replacement is not concurrent-safe.
func stubRuntimeEventsEmit(t *testing.T) {
	t.Helper()
	orig := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(ctx context.Context, name string, data ...any) {}
	t.Cleanup(func() { runtimeEventsEmitFn = orig })
}

// newSessionServiceForTest creates a session.Service with safe nil-guarded
// Deps closures for tests that construct *App directly via &App{...}.
// Call this after setting app.sessions, app.router, and app.configState.
func newSessionServiceForTest(app *App) *session.Service {
	return session.NewService(session.Deps{
		Emitter:        apptypes.NoopEmitter{},
		IsShuttingDown: func() bool { return false },
		RequireSessions: func() (*tmux.SessionManager, error) {
			return app.requireSessions()
		},
		RequireRouter: func() (*tmux.CommandRouter, error) {
			return app.requireRouter()
		},
		GetConfigSnapshot: func() config.Config {
			if app.configState == nil {
				return config.DefaultConfig()
			}
			return app.configState.Snapshot()
		},
		RuntimeContext: func() context.Context {
			return app.runtimeContext()
		},
		RequestSnapshot: func(force bool) {
			if app.snapshotService != nil {
				app.snapshotService.RequestSnapshot(force)
			}
		},
		EmitBackendEvent: func(name string, payload any) {
			// no-op in tests unless explicitly stubbed
		},
		McpCleanupSession: func(sessionName string) {
			if app.mcpManager != nil {
				app.mcpManager.CleanupSession(sessionName)
			}
		},
		CleanupStaleSnapshotState: func(activePaneIDs map[string]struct{}) {
			if app.snapshotService != nil {
				app.snapshotService.CleanupDetachedPaneStates(
					app.snapshotService.DetachStaleOutputBuffers(activePaneIDs),
				)
			}
		},
		ExecuteRouterRequest: func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			return router.Execute(req)
		},
	})
}

// waitForCondition polls condition at 5ms intervals until it returns true or
// the timeout expires. Used for async assertions on timer/goroutine behavior.
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timeout waiting for condition: %s", message)
		case <-ticker.C:
		}
	}
}
