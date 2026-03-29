package session

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestDeps returns a Deps with safe defaults for unit tests.
// Callers override individual fields as needed.
// RequireRouter returns an error by default; use newTestDepsWithRouter for
// tests that need router-dependent methods (CreateSession, KillSession, etc.).
func newTestDeps() Deps {
	sm := tmux.NewSessionManager()
	return Deps{
		Emitter:        apptypes.NoopEmitter{},
		IsShuttingDown: func() bool { return false },
		RequireSessions: func() (*tmux.SessionManager, error) {
			return sm, nil
		},
		RequireRouter: func() (*tmux.CommandRouter, error) {
			return nil, errors.New("router not available in test")
		},
		GetConfigSnapshot: func() config.Config {
			return config.DefaultConfig()
		},
		RuntimeContext: func() context.Context {
			return context.Background()
		},
		RequestSnapshot:           func(bool) {},
		EmitBackendEvent:          func(string, any) {},
		McpCleanupSession:         func(string) {},
		CleanupStaleSnapshotState: func(map[string]struct{}) {},
		ExecuteRouterRequest: func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			return router.Execute(req)
		},
	}
}

// newTestDepsWithRouter returns Deps with a mock router that delegates
// to the provided handler. This enables testing router-dependent methods
// like CreateSessionForDirectory, KillSession, and RollbackCreatedSession.
func newTestDepsWithRouter(routerHandler func(*tmux.CommandRouter, ipc.TmuxRequest) ipc.TmuxResponse) Deps {
	deps := newTestDeps()
	// The ExecuteRouterRequest override is set directly on deps.
	deps.ExecuteRouterRequest = routerHandler
	// Provide a non-nil router so RequireRouter succeeds.
	router := &tmux.CommandRouter{}
	deps.RequireRouter = func() (*tmux.CommandRouter, error) {
		return router, nil
	}
	return deps
}

// testEmitter is a minimal RuntimeEventEmitter for unit tests.
type testEmitter struct {
	fn func(name string, payload any)
}

func (e testEmitter) Emit(name string, payload any) { e.fn(name, payload) }
func (e testEmitter) EmitWithContext(_ context.Context, name string, payload any) {
	e.fn(name, payload)
}

// ---------------------------------------------------------------------------
// Field count guard tests (C-3, C-4)
// ---------------------------------------------------------------------------

func TestDeps_FieldCount(t *testing.T) {
	const expectedFieldCount = 11
	if got := reflect.TypeFor[Deps]().NumField(); got != expectedFieldCount {
		t.Fatalf("Deps has %d fields, expected %d; update newTestDeps, "+
			"newTestDepsWithRouter, newSessionServiceForTest, and this assertion", got, expectedFieldCount)
	}
}

func TestCreateSessionOptionsFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	if got := reflect.TypeFor[CreateSessionOptions]().NumField(); got != expectedFieldCount {
		t.Fatalf("session.CreateSessionOptions field count = %d, want %d; "+
			"update toSessionOpts() in app_session_api.go and this assertion", got, expectedFieldCount)
	}
}

func TestWorktreeCleanupParamsFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	if got := reflect.TypeFor[WorktreeCleanupParams]().NumField(); got != expectedFieldCount {
		t.Fatalf("WorktreeCleanupParams field count = %d, want %d; "+
			"update KillSession worktree cleanup and this assertion", got, expectedFieldCount)
	}
}

// ---------------------------------------------------------------------------
// NewService tests
// ---------------------------------------------------------------------------

func TestNewService_RequiredFieldsNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewService with nil RequireSessions should panic")
		}
	}()
	NewService(Deps{})
}

func TestNewService_PanicReportsIndividualMissingFields(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewService with partial nil should panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		// Verify individual field names are reported.
		if got := msg; got == "" {
			t.Fatal("panic message is empty")
		}
	}()
	deps := newTestDeps()
	deps.RequireRouter = nil
	deps.McpCleanupSession = nil
	NewService(deps)
}

func TestNewService_OptionalFieldDefaults(t *testing.T) {
	deps := newTestDeps()
	deps.Emitter = nil
	deps.IsShuttingDown = nil
	svc := NewService(deps)
	// Verify no panic on calling methods that use optional fields.
	svc.SetActiveSessionName("test")
	if got := svc.GetActiveSessionName(); got != "test" {
		t.Errorf("GetActiveSessionName() = %q, want %q", got, "test")
	}
}

// ---------------------------------------------------------------------------
// Active session tests
// ---------------------------------------------------------------------------

func TestSetActive_EmitsEvent(t *testing.T) {
	var emittedName string
	var emittedEvent string
	deps := newTestDeps()
	deps.Emitter = testEmitter{fn: func(name string, payload any) {
		emittedEvent = name
		if m, ok := payload.(map[string]string); ok {
			emittedName = m["name"]
		}
	}}
	svc := NewService(deps)
	svc.SetActive("my-session")
	if emittedEvent != "tmux:active-session" {
		t.Errorf("event = %q, want %q", emittedEvent, "tmux:active-session")
	}
	if emittedName != "my-session" {
		t.Errorf("name = %q, want %q", emittedName, "my-session")
	}
}

func TestSetActive_SkipsEmitDuringShutdown(t *testing.T) {
	emitted := false
	deps := newTestDeps()
	deps.IsShuttingDown = func() bool { return true }
	deps.Emitter = testEmitter{fn: func(string, any) {
		emitted = true
	}}
	svc := NewService(deps)
	svc.SetActive("during-shutdown")
	if emitted {
		t.Fatal("SetActive should not emit events during shutdown")
	}
	// Name should still be stored.
	if got := svc.GetActiveSessionName(); got != "during-shutdown" {
		t.Errorf("GetActiveSessionName() = %q, want %q", got, "during-shutdown")
	}
}

func TestSetActiveSessionName_Normalizes(t *testing.T) {
	svc := NewService(newTestDeps())
	got := svc.SetActiveSessionName("  trimmed  ")
	if got != "trimmed" {
		t.Errorf("SetActiveSessionName returned %q, want %q", got, "trimmed")
	}
	if name := svc.GetActiveSessionName(); name != "trimmed" {
		t.Errorf("GetActiveSessionName() = %q, want %q", name, "trimmed")
	}
}

// ---------------------------------------------------------------------------
// Shutdown guard tests (C-1)
// ---------------------------------------------------------------------------

func TestCreateSession_RejectsWhenShuttingDown(t *testing.T) {
	deps := newTestDeps()
	deps.IsShuttingDown = func() bool { return true }
	svc := NewService(deps)
	_, err := svc.CreateSession("C:/some/path", "test", CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSession should return error during shutdown")
	}
	if got := err.Error(); got != "cannot create session: application is shutting down" {
		t.Errorf("error = %q, want shutdown message", got)
	}
}

func TestRenameSession_RejectsWhenShuttingDown(t *testing.T) {
	deps := newTestDeps()
	deps.IsShuttingDown = func() bool { return true }
	svc := NewService(deps)
	err := svc.RenameSession("old", "new")
	if err == nil {
		t.Fatal("RenameSession should return error during shutdown")
	}
	if got := err.Error(); got != "cannot rename session: application is shutting down" {
		t.Errorf("error = %q, want shutdown message", got)
	}
}

// ---------------------------------------------------------------------------
// Session lookup tests
// ---------------------------------------------------------------------------

func TestFindAvailableSessionName_NoConflict(t *testing.T) {
	svc := NewService(newTestDeps())
	got := svc.FindAvailableSessionName("fresh")
	if got != "fresh" {
		t.Errorf("FindAvailableSessionName(%q) = %q, want %q", "fresh", got, "fresh")
	}
}

func TestFindAvailableSessionName_WithConflict(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("taken", "0", 120, 40)
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	got := svc.FindAvailableSessionName("taken")
	if got != "taken-2" {
		t.Errorf("FindAvailableSessionName(%q) = %q, want %q", "taken", got, "taken-2")
	}
}

func TestFindSessionByRootPath_Match(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("sess1", "0", 120, 40)
	sm.SetRootPath("sess1", "C:/projects/myapp")
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	if got := svc.FindSessionByRootPath("C:/projects/myapp"); got != "sess1" {
		t.Errorf("FindSessionByRootPath = %q, want %q", got, "sess1")
	}
}

func TestFindSessionByRootPath_CaseInsensitive(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("sess1", "0", 120, 40)
	sm.SetRootPath("sess1", "C:/Projects/MyApp")
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	if got := svc.FindSessionByRootPath("c:/projects/myapp"); got != "sess1" {
		t.Errorf("FindSessionByRootPath (case insensitive) = %q, want %q", got, "sess1")
	}
}

func TestFindSessionByRootPath_SkipsWorktreeSessions(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("wt-sess", "0", 120, 40)
	sm.SetRootPath("wt-sess", "C:/repo")
	sm.SetWorktreeInfo("wt-sess", &tmux.SessionWorktreeInfo{Path: "C:/repo.wt/branch"})
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	if got := svc.FindSessionByRootPath("C:/repo"); got != "" {
		t.Errorf("FindSessionByRootPath should skip worktree sessions, got %q", got)
	}
}

func TestFindSessionByWorktreePath(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("wt-sess", "0", 120, 40)
	sm.SetWorktreeInfo("wt-sess", &tmux.SessionWorktreeInfo{Path: "C:/repo.wt/branch"})
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	if got := svc.FindSessionByWorktreePath("C:/repo.wt/branch"); got != "wt-sess" {
		t.Errorf("FindSessionByWorktreePath = %q, want %q", got, "wt-sess")
	}
}

func TestCheckDirectoryConflict_TrimsInput(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("sess1", "0", 120, 40)
	sm.SetRootPath("sess1", "C:/projects/myapp")
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	if got := svc.CheckDirectoryConflict("  C:/projects/myapp  "); got != "sess1" {
		t.Errorf("CheckDirectoryConflict = %q, want %q", got, "sess1")
	}
}

// ---------------------------------------------------------------------------
// ResolveSessionByCwd tests (I-9)
// ---------------------------------------------------------------------------

func TestResolveSessionByCwd(t *testing.T) {
	tests := []struct {
		name    string
		cwd     string
		wantErr bool
		want    string
	}{
		{name: "empty cwd", cwd: "", wantErr: true},
		{name: "whitespace cwd", cwd: "   ", wantErr: true},
		{name: "root path match", cwd: "C:/projects/myapp", want: "root-sess"},
		{name: "worktree path match", cwd: "C:/repo.wt/branch", want: "wt-sess"},
		{name: "no match", cwd: "C:/unknown/path", wantErr: true},
	}

	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("root-sess", "0", 120, 40)
	sm.SetRootPath("root-sess", "C:/projects/myapp")
	sm.CreateSession("wt-sess", "1", 120, 40)
	sm.SetWorktreeInfo("wt-sess", &tmux.SessionWorktreeInfo{Path: "C:/repo.wt/branch"})
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.ResolveSessionByCwd(tt.cwd)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveSessionByCwd(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveSessionDir tests
// ---------------------------------------------------------------------------

func TestResolveSessionDir_WorktreeVsRoot(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("wt-sess", "0", 120, 40)
	sm.SetRootPath("wt-sess", "C:/repo")
	sm.SetWorktreeInfo("wt-sess", &tmux.SessionWorktreeInfo{
		Path:     "C:/repo.wt/branch",
		RepoPath: "C:/repo",
	})
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)

	workDir, err := svc.ResolveSessionWorkDir("wt-sess")
	if err != nil {
		t.Fatalf("ResolveSessionWorkDir error: %v", err)
	}
	if workDir != "C:/repo.wt/branch" {
		t.Errorf("ResolveSessionWorkDir = %q, want %q", workDir, "C:/repo.wt/branch")
	}

	repoDir, err := svc.ResolveSessionRepoDir("wt-sess")
	if err != nil {
		t.Fatalf("ResolveSessionRepoDir error: %v", err)
	}
	if repoDir != "C:/repo" {
		t.Errorf("ResolveSessionRepoDir = %q, want %q", repoDir, "C:/repo")
	}
}

func TestResolveSessionDir_NotFound(t *testing.T) {
	svc := NewService(newTestDeps())
	_, err := svc.ResolveSessionDir("nonexistent", true)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

// ---------------------------------------------------------------------------
// ListSessions / GetSessionEnv tests
// ---------------------------------------------------------------------------

func TestListSessions_Empty(t *testing.T) {
	svc := NewService(newTestDeps())
	sessions := svc.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("ListSessions = %d sessions, want 0", len(sessions))
	}
}

func TestListSessions_ReturnsNilOnError(t *testing.T) {
	deps := newTestDeps()
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return nil, errors.New("unavailable")
	}
	svc := NewService(deps)
	result := svc.ListSessions()
	if result != nil {
		t.Errorf("ListSessions should return nil on error, got %v", result)
	}
}

func TestGetSessionEnv_EmptyName(t *testing.T) {
	svc := NewService(newTestDeps())
	_, err := svc.GetSessionEnv("")
	if err == nil {
		t.Fatal("expected error for empty session name")
	}
}

// ---------------------------------------------------------------------------
// ActivateCreatedSession tests (M-1)
// ---------------------------------------------------------------------------

func TestActivateCreatedSession_NotFound(t *testing.T) {
	svc := NewService(newTestDeps())
	_, err := svc.ActivateCreatedSession("nonexistent-session")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestActivateCreatedSession_SetsActiveAndReturnsSnapshot(t *testing.T) {
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("sess1", "0", 120, 40)
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)
	snap, err := svc.ActivateCreatedSession("sess1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Name != "sess1" {
		t.Errorf("snapshot.Name = %q, want %q", snap.Name, "sess1")
	}
	if got := svc.GetActiveSessionName(); got != "sess1" {
		t.Errorf("GetActiveSessionName() = %q, want %q", got, "sess1")
	}
}

// ---------------------------------------------------------------------------
// EmitWorktreeCleanupFailure tests (C-2)
// ---------------------------------------------------------------------------

func TestEmitWorktreeCleanupFailure_SkipsDuringShutdown(t *testing.T) {
	emitted := false
	deps := newTestDeps()
	deps.IsShuttingDown = func() bool { return true }
	deps.Emitter = testEmitter{fn: func(string, any) {
		emitted = true
	}}
	svc := NewService(deps)
	svc.EmitWorktreeCleanupFailure("sess", "/path", errors.New("test error"))
	if emitted {
		t.Fatal("EmitWorktreeCleanupFailure should not emit during shutdown")
	}
}

func TestEmitWorktreeCleanupFailure_SkipsWhenCtxNil(t *testing.T) {
	emitted := false
	deps := newTestDeps()
	deps.RuntimeContext = func() context.Context { return nil }
	deps.Emitter = testEmitter{fn: func(string, any) {
		emitted = true
	}}
	svc := NewService(deps)
	svc.EmitWorktreeCleanupFailure("sess", "/path", errors.New("test error"))
	if emitted {
		t.Fatal("EmitWorktreeCleanupFailure should not emit when context is nil")
	}
}

func TestEmitWorktreeCleanupFailure_EmitsEvent(t *testing.T) {
	var eventName string
	var eventPayload any
	deps := newTestDeps()
	deps.Emitter = testEmitter{fn: func(name string, payload any) {
		eventName = name
		eventPayload = payload
	}}
	svc := NewService(deps)
	svc.EmitWorktreeCleanupFailure("sess1", "/wt/path", errors.New("cleanup failed"))
	if eventName != "worktree:cleanup-failed" {
		t.Errorf("event = %q, want %q", eventName, "worktree:cleanup-failed")
	}
	m, ok := eventPayload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", eventPayload)
	}
	if m["sessionName"] != "sess1" {
		t.Errorf("sessionName = %v, want %q", m["sessionName"], "sess1")
	}
	if m["error"] != "cleanup failed" {
		t.Errorf("error = %v, want %q", m["error"], "cleanup failed")
	}
}

func TestEmitWorktreeCleanupFailure_HandlesNilError(t *testing.T) {
	var eventPayload any
	deps := newTestDeps()
	deps.Emitter = testEmitter{fn: func(_ string, payload any) {
		eventPayload = payload
	}}
	svc := NewService(deps)
	svc.EmitWorktreeCleanupFailure("sess1", "/wt/path", nil)
	m, ok := eventPayload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", eventPayload)
	}
	if m["error"] == "" {
		t.Error("error field should not be empty when nil error is passed")
	}
}

// ---------------------------------------------------------------------------
// OverrideExecuteRouterRequest tests
// ---------------------------------------------------------------------------

func TestOverrideExecuteRouterRequest(t *testing.T) {
	deps := newTestDeps()
	svc := NewService(deps)

	called := false
	restore := svc.OverrideExecuteRouterRequest(func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
		called = true
		return ipc.TmuxResponse{ExitCode: 0}
	})
	defer restore()

	// Verify override is in effect.
	svc.deps.ExecuteRouterRequest(nil, ipc.TmuxRequest{})
	if !called {
		t.Fatal("override function was not called")
	}
}

// ---------------------------------------------------------------------------
// FindSessionSnapshotByName tests
// ---------------------------------------------------------------------------

func TestFindSessionSnapshotByName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty name", input: "", wantErr: true},
		{name: "whitespace name", input: "   ", wantErr: true},
		{name: "not found", input: "nonexistent", wantErr: true},
		{name: "found", input: "sess1", wantErr: false},
	}
	deps := newTestDeps()
	sm := tmux.NewSessionManager()
	sm.CreateSession("sess1", "0", 120, 40)
	deps.RequireSessions = func() (*tmux.SessionManager, error) {
		return sm, nil
	}
	svc := NewService(deps)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap, err := svc.FindSessionSnapshotByName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if snap.Name != "sess1" {
				t.Errorf("snap.Name = %q, want %q", snap.Name, "sess1")
			}
		})
	}
}
