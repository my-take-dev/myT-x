package mcp

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestManager creates a Manager with a pre-populated registry and a
// thread-safe event collector for assertions.
func newTestManager(t *testing.T, defs ...MCPDefinition) (*Manager, *eventCollector) {
	t.Helper()
	reg := NewRegistry()
	for _, d := range defs {
		if strings.TrimSpace(d.Command) == "" {
			d.Command = "test-command"
		}
		if err := reg.Register(d); err != nil {
			t.Fatalf("newTestManager: Register(%q): %v", d.ID, err)
		}
	}
	ec := &eventCollector{}
	mgr := NewManager(ManagerConfig{Registry: reg, EmitFn: ec.emit})
	return mgr, ec
}

type eventCollector struct {
	mu     sync.Mutex
	events []emittedEvent
}

type emittedEvent struct {
	Name    string
	Payload any
}

func (ec *eventCollector) emit(name string, payload any) {
	ec.mu.Lock()
	ec.events = append(ec.events, emittedEvent{Name: name, Payload: payload})
	ec.mu.Unlock()
}

func (ec *eventCollector) count() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.events)
}

func (ec *eventCollector) last() emittedEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.events) == 0 {
		return emittedEvent{}
	}
	return ec.events[len(ec.events)-1]
}

func (ec *eventCollector) reset() {
	ec.mu.Lock()
	ec.events = nil
	ec.mu.Unlock()
}

func payloadMap(t *testing.T, payload any) map[string]any {
	t.Helper()
	data, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", payload)
	}
	return data
}

func TestNewManagerPanicsWhenOrchestratorConfigDirIsMissing(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{
		ID:   "agent-orchestrator",
		Name: "Agent Orchestrator",
		Kind: DefinitionKindOrchestrator,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("NewManager() expected panic")
		}
		msg, ok := got.(string)
		if !ok {
			t.Fatalf("NewManager() panic = %T(%v), want string", got, got)
		}
		if !strings.Contains(msg, "ConfigDir is required") {
			t.Fatalf("NewManager() panic = %v, want ConfigDir requirement", got)
		}
	}()
	_ = NewManager(ManagerConfig{
		Registry: reg,
		EmitFn:   func(string, any) {},
	})
}

type fakeManagedPipeServer struct {
	pipeName     string
	startEntered chan struct{}
	startRelease chan struct{}
	startErr     error
	stopErr      error

	startOnce sync.Once
	mu        sync.Mutex
	started   bool
	stopCount int
}

func (f *fakeManagedPipeServer) Start() error {
	f.startOnce.Do(func() {
		close(f.startEntered)
	})
	if f.startRelease != nil {
		<-f.startRelease
	}
	if f.startErr != nil {
		return f.startErr
	}
	f.mu.Lock()
	f.started = true
	f.mu.Unlock()
	return nil
}

func (f *fakeManagedPipeServer) Stop() error {
	f.mu.Lock()
	f.stopCount++
	f.mu.Unlock()
	return f.stopErr
}

func (f *fakeManagedPipeServer) PipeName() string {
	return f.pipeName
}

func (f *fakeManagedPipeServer) snapshot() (started bool, stopCount int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started, f.stopCount
}

func mustReceiveWithin(t *testing.T, ch <-chan struct{}, timeout time.Duration, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(message)
	}
}

func mustNotReceiveWithin(t *testing.T, ch <-chan struct{}, timeout time.Duration, message string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(message)
	case <-time.After(timeout):
	}
}

func TestManager_SnapshotForSession(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		defs        []MCPDefinition
		wantLen     int
		wantErr     bool
	}{
		{
			name:        "empty session name",
			sessionName: "",
			wantErr:     true,
		},
		{
			name:        "whitespace session name",
			sessionName: "   ",
			wantErr:     true,
		},
		{
			name:        "empty registry",
			sessionName: "session-1",
			defs:        nil,
			wantLen:     0,
		},
		{
			name:        "returns all definitions",
			sessionName: "session-1",
			defs: []MCPDefinition{
				{ID: "memory", Name: "Memory"},
				{ID: "browser", Name: "Browser"},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _ := newTestManager(t, tt.defs...)
			snapshots, err := mgr.SnapshotForSession(tt.sessionName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SnapshotForSession(%q) error = %v, wantErr %v", tt.sessionName, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(snapshots) != tt.wantLen {
				t.Errorf("SnapshotForSession(%q) = %d snapshots, want %d", tt.sessionName, len(snapshots), tt.wantLen)
			}
			// All snapshots should be disabled by default.
			for _, snap := range snapshots {
				if snap.Enabled {
					t.Errorf("SnapshotForSession(%q): %q should be disabled by default", tt.sessionName, snap.ID)
				}
				if snap.Status != StatusStopped {
					t.Errorf("SnapshotForSession(%q): %q status = %q, want %q", tt.sessionName, snap.ID, snap.Status, StatusStopped)
				}
			}
		})
	}
}

func TestManager_SnapshotForSession_DefaultEnabledSnapshot(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{
		ID:             "memory",
		Name:           "Memory",
		DefaultEnabled: true,
	})
	snapshots, err := mgr.SnapshotForSession("session-1")
	if err != nil {
		t.Fatalf("SnapshotForSession() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("SnapshotForSession() length = %d, want 1", len(snapshots))
	}
	if !snapshots[0].Enabled {
		t.Fatal("default-enabled MCP snapshot should be enabled")
	}
}

func TestManager_SetEnabled(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		mcpID       string
		enabled     bool
		wantEvent   bool
		wantErr     bool
	}{
		{
			name:        "enable MCP",
			sessionName: "session-1",
			mcpID:       "memory",
			enabled:     true,
			wantEvent:   true,
		},
		{
			name:        "disable MCP no-op when already at default state",
			sessionName: "session-1",
			mcpID:       "memory",
			enabled:     false,
			wantEvent:   false,
		},
		{
			name:        "empty session name",
			sessionName: "",
			mcpID:       "memory",
			wantErr:     true,
		},
		{
			name:        "empty MCP ID",
			sessionName: "session-1",
			mcpID:       "",
			wantErr:     true,
		},
		{
			name:        "unknown MCP ID",
			sessionName: "session-1",
			mcpID:       "unknown",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, ec := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
			err := mgr.SetEnabled(tt.sessionName, tt.mcpID, tt.enabled)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetEnabled(%q, %q, %v) error = %v, wantErr %v",
					tt.sessionName, tt.mcpID, tt.enabled, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// Verify state change.
			snapshots, err := mgr.SnapshotForSession(tt.sessionName)
			if err != nil {
				t.Fatalf("SnapshotForSession failed: %v", err)
			}
			found := false
			for _, snap := range snapshots {
				if snap.ID == tt.mcpID {
					found = true
					if snap.Enabled != tt.enabled {
						t.Errorf("after SetEnabled: Enabled = %v, want %v", snap.Enabled, tt.enabled)
					}
				}
			}
			if !found {
				t.Errorf("after SetEnabled: MCP %q not found in snapshot", tt.mcpID)
			}

			// Verify event emission only on state-changing updates.
			if tt.wantEvent && ec.count() == 0 {
				t.Error("SetEnabled did not emit mcp:state-changed event")
			}
			if !tt.wantEvent && ec.count() != 0 {
				t.Errorf("SetEnabled emitted unexpected event count = %d, want 0", ec.count())
			}
			if tt.wantEvent {
				last := ec.last()
				if last.Name != "mcp:state-changed" {
					t.Fatalf("last event name = %q, want %q", last.Name, "mcp:state-changed")
				}
				payload := payloadMap(t, last.Payload)
				if got := payload["session_name"]; got != tt.sessionName {
					t.Fatalf("event payload session_name = %v, want %q", got, tt.sessionName)
				}
				if got := payload["mcp_id"]; got != tt.mcpID {
					t.Fatalf("event payload mcp_id = %v, want %q", got, tt.mcpID)
				}
			}
		})
	}
}

func TestManager_SetEnabled_NoOpForExistingSameValue(t *testing.T) {
	mgr, ec := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	ec.reset()

	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("second SetEnabled(true) error = %v", err)
	}
	if ec.count() != 0 {
		t.Fatalf("second SetEnabled(true) emitted %d events, want 0", ec.count())
	}
}

func TestManager_SetEnabled_DefaultEnabledTrueNoOp(t *testing.T) {
	mgr, ec := newTestManager(t, MCPDefinition{
		ID:             "memory",
		Name:           "Memory",
		DefaultEnabled: true,
	})
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	if ec.count() != 0 {
		t.Fatalf("SetEnabled(true) emitted %d events, want 0 for default-enabled no-op", ec.count())
	}
}

func TestManager_SetEnabled_DisableDefaultEnabledCreatesOverride(t *testing.T) {
	mgr, ec := newTestManager(t, MCPDefinition{
		ID:             "memory",
		Name:           "Memory",
		DefaultEnabled: true,
	})
	if err := mgr.SetEnabled("session-1", "memory", false); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	snapshots, err := mgr.SnapshotForSession("session-1")
	if err != nil {
		t.Fatalf("SnapshotForSession() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("SnapshotForSession() length = %d, want 1", len(snapshots))
	}
	if snapshots[0].Enabled {
		t.Fatal("SnapshotForSession().Enabled = true, want false after override")
	}
	if ec.count() == 0 {
		t.Fatal("SetEnabled() did not emit state-changed event for default-enabled override")
	}
}

func TestManager_SetEnabled_ClosedManager(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	mgr.Close()

	err := mgr.SetEnabled("session-1", "memory", true)
	if err == nil {
		t.Fatal("SetEnabled on closed manager should return error")
	}
}

func TestManager_GetDetail(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		mcpID       string
		wantErr     bool
	}{
		{
			name:        "existing MCP",
			sessionName: "session-1",
			mcpID:       "memory",
		},
		{
			name:        "empty session name",
			sessionName: "",
			mcpID:       "memory",
			wantErr:     true,
		},
		{
			name:        "empty MCP ID",
			sessionName: "session-1",
			mcpID:       "",
			wantErr:     true,
		},
		{
			name:        "unknown MCP ID",
			sessionName: "session-1",
			mcpID:       "unknown",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _ := newTestManager(t, MCPDefinition{
				ID:          "memory",
				Name:        "Memory Server",
				Description: "test description",
				UsageSample: "sample",
			})

			detail, err := mgr.GetDetail(tt.sessionName, tt.mcpID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetDetail(%q, %q) error = %v, wantErr %v",
					tt.sessionName, tt.mcpID, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if detail.Name != "Memory Server" {
				t.Errorf("GetDetail Name = %q, want %q", detail.Name, "Memory Server")
			}
			if detail.UsageSample != "sample" {
				t.Errorf("GetDetail UsageSample = %q, want %q", detail.UsageSample, "sample")
			}
		})
	}
}

func TestManager_SnapshotForSession_ClosedManager(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	mgr.Close()

	_, err := mgr.SnapshotForSession("session-1")
	if err == nil {
		t.Fatal("SnapshotForSession() on closed manager should return error")
	}
}

func TestManager_GetDetail_ReflectsEnabledState(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})

	// Initially disabled.
	detail, err := mgr.GetDetail("session-1", "memory")
	if err != nil {
		t.Fatalf("GetDetail failed: %v", err)
	}
	if detail.Enabled {
		t.Error("GetDetail: should be disabled initially")
	}

	// Enable.
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}

	detail, err = mgr.GetDetail("session-1", "memory")
	if err != nil {
		t.Fatalf("GetDetail failed: %v", err)
	}
	if !detail.Enabled {
		t.Error("GetDetail: should be enabled after SetEnabled(true)")
	}

	// Disable and verify.
	if err := mgr.SetEnabled("session-1", "memory", false); err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	detail, err = mgr.GetDetail("session-1", "memory")
	if err != nil {
		t.Fatalf("GetDetail failed: %v", err)
	}
	if detail.Enabled {
		t.Error("GetDetail: should be disabled after SetEnabled(false)")
	}

	// Re-enable and verify again.
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("second SetEnabled(true) failed: %v", err)
	}
	detail, err = mgr.GetDetail("session-1", "memory")
	if err != nil {
		t.Fatalf("GetDetail failed: %v", err)
	}
	if !detail.Enabled {
		t.Error("GetDetail: should be enabled after enable->disable->enable")
	}
}

func TestManager_GetDetail_ClosedManager(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	mgr.Close()

	_, err := mgr.GetDetail("session-1", "memory")
	if err == nil {
		t.Fatal("GetDetail() on closed manager should return error")
	}
}

func TestManager_CleanupSession(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})

	// Setup session state.
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}

	// Cleanup.
	if err := mgr.CleanupSession("session-1"); err != nil {
		t.Fatalf("CleanupSession() error = %v", err)
	}

	// After cleanup, snapshot should show default state.
	snapshots, err := mgr.SnapshotForSession("session-1")
	if err != nil {
		t.Fatalf("SnapshotForSession failed: %v", err)
	}
	for _, snap := range snapshots {
		if snap.Enabled {
			t.Errorf("after CleanupSession: %q should be disabled", snap.ID)
		}
	}
}

func TestManager_CleanupSession_EmptyName(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	// Should not panic.
	if err := mgr.CleanupSession(""); err != nil {
		t.Fatalf("CleanupSession(\"\") error = %v", err)
	}
	if err := mgr.CleanupSession("   "); err != nil {
		t.Fatalf("CleanupSession(blank) error = %v", err)
	}
}

func TestManager_CleanupSession_NonExistingSession(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	// Should not panic on missing sessions.
	if err := mgr.CleanupSession("never-created"); err != nil {
		t.Fatalf("CleanupSession(non-existing) error = %v", err)
	}
}

func TestManager_CleanupSession_CallsCancel(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	mgr.mu.RLock()
	inst := mgr.sessions["session-1"]["memory"]
	mgr.mu.RUnlock()
	if inst == nil {
		t.Fatal("session instance not found after SetEnabled")
	}

	cancelCount := 0
	inst.mu.Lock()
	inst.cancel = func() error {
		cancelCount++
		return nil
	}
	inst.mu.Unlock()

	if err := mgr.CleanupSession("session-1"); err != nil {
		t.Fatalf("CleanupSession() error = %v", err)
	}
	if cancelCount != 1 {
		t.Fatalf("CleanupSession() cancel count = %d, want 1", cancelCount)
	}
}

func TestManager_CleanupSession_ReturnsStopErrors(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	mgr.mu.RLock()
	inst := mgr.sessions["session-1"]["memory"]
	mgr.mu.RUnlock()
	if inst == nil {
		t.Fatal("session instance not found after SetEnabled")
	}

	inst.mu.Lock()
	inst.cancel = func() error { return errors.New("stop failed") }
	inst.mu.Unlock()

	err := mgr.CleanupSession("session-1")
	if err == nil {
		t.Fatal("CleanupSession() expected stop error")
	}
	if !strings.Contains(err.Error(), "stop mcp \"memory\"") {
		t.Fatalf("CleanupSession() error = %v, want stop context", err)
	}
}

func TestManager_RenameSession_MigratesPreservedState(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})

	mgr.mu.Lock()
	mgr.sessions["session-old"] = map[string]*instance{
		"memory": {
			state: InstanceState{
				MCPID:     "memory",
				SessionID: "session-old",
				Enabled:   true,
				Status:    StatusError,
				Error:     "startup failed",
			},
		},
	}
	mgr.mu.Unlock()

	if err := mgr.RenameSession("session-old", "session-new"); err != nil {
		t.Fatalf("RenameSession() error = %v", err)
	}

	mgr.mu.RLock()
	_, oldExists := mgr.sessions["session-old"]
	newInst := mgr.sessions["session-new"]["memory"]
	mgr.mu.RUnlock()
	if oldExists {
		t.Fatal("old session entry should be removed after rename")
	}
	if newInst == nil {
		t.Fatal("new session entry should exist after rename")
	}

	newInst.mu.RLock()
	defer newInst.mu.RUnlock()
	if newInst.state.SessionID != "session-new" {
		t.Fatalf("SessionID = %q, want %q", newInst.state.SessionID, "session-new")
	}
	if !newInst.state.Enabled {
		t.Fatal("Enabled should be preserved after rename")
	}
	if newInst.state.Status != StatusError {
		t.Fatalf("Status = %q, want %q", newInst.state.Status, StatusError)
	}
	if newInst.state.Error != "startup failed" {
		t.Fatalf("Error = %q, want %q", newInst.state.Error, "startup failed")
	}
}

func TestManager_RenameSession_ValidatesInputAndRejectsConflicts(t *testing.T) {
	t.Run("rejects empty old session name", func(t *testing.T) {
		mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
		if err := mgr.RenameSession("", "session-new"); err == nil || err.Error() != "old session name is required" {
			t.Fatalf("RenameSession() error = %v", err)
		}
	})

	t.Run("rejects empty new session name", func(t *testing.T) {
		mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
		if err := mgr.RenameSession("session-old", ""); err == nil || err.Error() != "new session name is required" {
			t.Fatalf("RenameSession() error = %v", err)
		}
	})

	t.Run("same session name is a no-op", func(t *testing.T) {
		mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
		if err := mgr.RenameSession("session-old", "session-old"); err != nil {
			t.Fatalf("RenameSession() error = %v", err)
		}
	})

	t.Run("rejects existing target session state", func(t *testing.T) {
		mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
		mgr.mu.Lock()
		mgr.sessions["session-old"] = map[string]*instance{"memory": {state: InstanceState{MCPID: "memory", SessionID: "session-old"}}}
		mgr.sessions["session-new"] = map[string]*instance{"memory": {state: InstanceState{MCPID: "memory", SessionID: "session-new"}}}
		mgr.mu.Unlock()
		if err := mgr.RenameSession("session-old", "session-new"); err == nil || err.Error() != `session "session-new" already has MCP state` {
			t.Fatalf("RenameSession() error = %v", err)
		}
	})
}

func TestManager_RenameSession_RestartsRunningInstance(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var (
		pipesMu sync.Mutex
		pipes   []*fakeManagedPipeServer
	)
	newPipeServer := func(cfg MCPPipeConfig) managedPipeServer {
		pipe := &fakeManagedPipeServer{
			pipeName:     cfg.PipeName,
			startEntered: make(chan struct{}),
		}
		pipesMu.Lock()
		pipes = append(pipes, pipe)
		pipesMu.Unlock()
		return pipe
	}

	ec := &eventCollector{}
	mgr := NewManager(ManagerConfig{
		Registry:       reg,
		EmitFn:         ec.emit,
		ResolveWorkDir: func(string) (string, error) { return ".", nil },
		NewPipeServer:  newPipeServer,
	})

	if err := mgr.SetEnabled("session-old", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	var firstPipe *fakeManagedPipeServer
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pipesMu.Lock()
		if len(pipes) >= 1 {
			firstPipe = pipes[0]
		}
		pipesMu.Unlock()
		if firstPipe != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if firstPipe == nil {
		t.Fatal("initial pipe was not created")
	}
	mustReceiveWithin(t, firstPipe.startEntered, time.Second, "initial pipe Start was not called")

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		detail, err := mgr.GetDetail("session-old", "memory")
		if err == nil && detail.Status == StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := mgr.RenameSession("session-old", "session-new"); err != nil {
		t.Fatalf("RenameSession() error = %v", err)
	}

	var secondPipe *fakeManagedPipeServer
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pipesMu.Lock()
		if len(pipes) >= 2 {
			secondPipe = pipes[1]
		}
		pipesMu.Unlock()
		if secondPipe != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if secondPipe == nil {
		t.Fatal("renamed session pipe was not created")
	}
	mustReceiveWithin(t, secondPipe.startEntered, time.Second, "renamed session pipe Start was not called")

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, stopCount := firstPipe.snapshot()
		if stopCount == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, stopCount := firstPipe.snapshot(); stopCount != 1 {
		t.Fatalf("old pipe stop count = %d, want 1", stopCount)
	}

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		detail, err := mgr.GetDetail("session-new", "memory")
		if err == nil && detail.Status == StatusRunning && detail.PipePath == BuildMCPPipeName("session-new", "memory") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	detail, err := mgr.GetDetail("session-new", "memory")
	if err != nil {
		t.Fatalf("GetDetail(session-new) error = %v", err)
	}
	t.Fatalf("renamed detail = %+v, want running state on pipe %q", detail, BuildMCPPipeName("session-new", "memory"))
}

func TestManager_RenameSession_RollsBackWhenRestartFails(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var (
		pipesMu sync.Mutex
		pipes   []*fakeManagedPipeServer
	)
	newPipeServer := func(cfg MCPPipeConfig) managedPipeServer {
		pipe := &fakeManagedPipeServer{
			pipeName:     cfg.PipeName,
			startEntered: make(chan struct{}),
		}
		pipesMu.Lock()
		switch len(pipes) {
		case 1:
			pipe.startErr = errors.New("rename restart failed")
		}
		pipes = append(pipes, pipe)
		pipesMu.Unlock()
		return pipe
	}

	ec := &eventCollector{}
	mgr := NewManager(ManagerConfig{
		Registry:       reg,
		EmitFn:         ec.emit,
		ResolveWorkDir: func(string) (string, error) { return ".", nil },
		NewPipeServer:  newPipeServer,
	})

	if err := mgr.SetEnabled("session-old", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	var firstPipe *fakeManagedPipeServer
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pipesMu.Lock()
		if len(pipes) >= 1 {
			firstPipe = pipes[0]
		}
		pipesMu.Unlock()
		if firstPipe != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if firstPipe == nil {
		t.Fatal("initial pipe was not created")
	}
	mustReceiveWithin(t, firstPipe.startEntered, time.Second, "initial pipe Start was not called")

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		detail, err := mgr.GetDetail("session-old", "memory")
		if err == nil && detail.Status == StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	err := mgr.RenameSession("session-old", "session-new")
	if err == nil {
		t.Fatal("RenameSession() expected restart failure")
	}
	if !strings.Contains(err.Error(), "restart renamed MCP") {
		t.Fatalf("RenameSession() error = %v, want restart failure", err)
	}

	var secondPipe, thirdPipe *fakeManagedPipeServer
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pipesMu.Lock()
		if len(pipes) >= 2 {
			secondPipe = pipes[1]
		}
		if len(pipes) >= 3 {
			thirdPipe = pipes[2]
		}
		pipesMu.Unlock()
		if secondPipe != nil && thirdPipe != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if secondPipe == nil || thirdPipe == nil {
		t.Fatalf("pipe count = %d, want 3 after rollback", len(pipes))
	}
	mustReceiveWithin(t, secondPipe.startEntered, time.Second, "rename restart pipe Start was not called")
	mustReceiveWithin(t, thirdPipe.startEntered, time.Second, "rollback pipe Start was not called")

	if _, stopCount := firstPipe.snapshot(); stopCount != 1 {
		t.Fatalf("old pipe stop count = %d, want 1", stopCount)
	}
	if _, stopCount := secondPipe.snapshot(); stopCount != 1 {
		t.Fatalf("failed rename pipe stop count = %d, want 1", stopCount)
	}

	mgr.mu.RLock()
	_, oldExists := mgr.sessions["session-old"]
	_, newExists := mgr.sessions["session-new"]
	mgr.mu.RUnlock()
	if !oldExists {
		t.Fatal("old session entry should be restored after rollback")
	}
	if newExists {
		t.Fatal("new session entry should not remain after rollback")
	}
	if ec.count() < 2 {
		t.Fatalf("event count = %d, want at least 2", ec.count())
	}
	last := ec.last()
	if last.Name != "mcp:state-changed" {
		t.Fatalf("last event name = %q, want %q", last.Name, "mcp:state-changed")
	}
	payload := payloadMap(t, last.Payload)
	if got := payload["session_name"]; got != "session-old" {
		t.Fatalf("last event session_name = %v, want %q", got, "session-old")
	}
	if got := payload["mcp_id"]; got != "memory" {
		t.Fatalf("last event mcp_id = %v, want %q", got, "memory")
	}

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		detail, detailErr := mgr.GetDetail("session-old", "memory")
		if detailErr == nil && detail.Status == StatusRunning && detail.PipePath == BuildMCPPipeName("session-old", "memory") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	detail, detailErr := mgr.GetDetail("session-old", "memory")
	if detailErr != nil {
		t.Fatalf("GetDetail(session-old) error = %v", detailErr)
	}
	t.Fatalf("rolled back detail = %+v, want running state on pipe %q", detail, BuildMCPPipeName("session-old", "memory"))
}

func TestManager_RenameSession_BlocksConcurrentSetEnabled(t *testing.T) {
	// Verify that SetEnabled waits for a concurrent rename to finish instead
	// of racing with the restart / rollback window.
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	startGate := make(chan struct{}) // blocks pipe Start until we release it
	newPipeServer := func(cfg MCPPipeConfig) managedPipeServer {
		return &fakeManagedPipeServer{
			pipeName:     cfg.PipeName,
			startEntered: make(chan struct{}),
			startRelease: startGate,
		}
	}

	mgr := NewManager(ManagerConfig{
		Registry:       reg,
		EmitFn:         func(string, any) {},
		ResolveWorkDir: func(string) (string, error) { return ".", nil },
		NewPipeServer:  newPipeServer,
	})

	// Enable the MCP so there is something to restart during rename.
	close(startGate) // let the initial Start through
	if err := mgr.SetEnabled("session-old", "memory", true); err != nil {
		t.Fatalf("SetEnabled(old, true) error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		detail, err := mgr.GetDetail("session-old", "memory")
		if err == nil && detail.Status == StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Now use a blocking gate for the rename-triggered restart.
	renameGate := make(chan struct{})
	mgr.newPipeServer = func(cfg MCPPipeConfig) managedPipeServer {
		return &fakeManagedPipeServer{
			pipeName:     cfg.PipeName,
			startEntered: make(chan struct{}),
			startRelease: renameGate,
		}
	}

	renameDone := make(chan error, 1)
	go func() {
		renameDone <- mgr.RenameSession("session-old", "session-new")
	}()

	// Give rename a moment to acquire the renaming guard.
	time.Sleep(50 * time.Millisecond)

	// SetEnabled on the new session should block until rename completes.
	setEnabledDone := make(chan error, 1)
	go func() {
		setEnabledDone <- mgr.SetEnabled("session-new", "memory", false)
	}()

	// SetEnabled should NOT complete while rename is in progress.
	select {
	case err := <-setEnabledDone:
		t.Fatalf("SetEnabled completed before rename finished (err=%v); expected it to block", err)
	case <-time.After(100 * time.Millisecond):
		// OK, it is blocking as expected.
	}

	// Release the rename pipe start so rename can complete.
	close(renameGate)
	select {
	case err := <-renameDone:
		if err != nil {
			t.Fatalf("RenameSession() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RenameSession() timed out")
	}

	// Now SetEnabled should unblock.
	select {
	case err := <-setEnabledDone:
		if err != nil {
			t.Fatalf("SetEnabled(new, false) error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SetEnabled() timed out after rename completed")
	}
}

func TestManager_RenameSession_BlocksConcurrentCleanupOnOldSession(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	rollbackGate := make(chan struct{})
	var pipesMu sync.Mutex
	var pipes []*fakeManagedPipeServer
	newPipeServer := func(cfg MCPPipeConfig) managedPipeServer {
		pipe := &fakeManagedPipeServer{
			pipeName:     cfg.PipeName,
			startEntered: make(chan struct{}),
		}

		pipesMu.Lock()
		switch len(pipes) {
		case 1:
			pipe.startErr = errors.New("rename restart failed")
		case 2:
			pipe.startRelease = rollbackGate
		}
		pipes = append(pipes, pipe)
		pipesMu.Unlock()
		return pipe
	}

	mgr := NewManager(ManagerConfig{
		Registry:       reg,
		EmitFn:         func(string, any) {},
		ResolveWorkDir: func(string) (string, error) { return ".", nil },
		NewPipeServer:  newPipeServer,
	})

	if err := mgr.SetEnabled("session-old", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	var firstPipe *fakeManagedPipeServer
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pipesMu.Lock()
		if len(pipes) >= 1 {
			firstPipe = pipes[0]
		}
		pipesMu.Unlock()
		if firstPipe != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if firstPipe == nil {
		t.Fatal("initial pipe was not created")
	}
	mustReceiveWithin(t, firstPipe.startEntered, time.Second, "initial pipe Start was not called")

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		detail, err := mgr.GetDetail("session-old", "memory")
		if err == nil && detail.Status == StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	renameDone := make(chan error, 1)
	go func() {
		renameDone <- mgr.RenameSession("session-old", "session-new")
	}()

	var rollbackPipe *fakeManagedPipeServer
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pipesMu.Lock()
		if len(pipes) >= 3 {
			rollbackPipe = pipes[2]
		}
		pipesMu.Unlock()
		if rollbackPipe != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rollbackPipe == nil {
		t.Fatal("rollback pipe was not created")
	}
	mustReceiveWithin(t, rollbackPipe.startEntered, time.Second, "rollback pipe Start was not called")

	cleanupDone := make(chan struct{})
	go func() {
		if err := mgr.CleanupSession("session-old"); err != nil {
			t.Errorf("CleanupSession() error = %v", err)
		}
		close(cleanupDone)
	}()

	mustNotReceiveWithin(t, cleanupDone, 100*time.Millisecond, "CleanupSession(old) should block until rename completes")

	close(rollbackGate)
	select {
	case err := <-renameDone:
		if err == nil || !strings.Contains(err.Error(), "restart renamed MCP") {
			t.Fatalf("RenameSession() error = %v, want restart failure", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RenameSession() timed out")
	}

	mustReceiveWithin(t, cleanupDone, time.Second, "CleanupSession(old) did not resume after rename completed")
}

func TestManager_Close_CallsCancel(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	mgr.mu.RLock()
	inst := mgr.sessions["session-1"]["memory"]
	mgr.mu.RUnlock()
	if inst == nil {
		t.Fatal("session instance not found after SetEnabled")
	}

	cancelCount := 0
	inst.mu.Lock()
	inst.cancel = func() error {
		cancelCount++
		return nil
	}
	inst.mu.Unlock()

	mgr.CloseWithoutEvent()
	if cancelCount != 1 {
		t.Fatalf("CloseWithoutEvent() cancel count = %d, want 1", cancelCount)
	}
}

func TestManager_Close(t *testing.T) {
	mgr, ec := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})

	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}
	ec.reset()

	mgr.Close()
	if ec.count() != 1 {
		t.Fatalf("Close() event count = %d, want 1", ec.count())
	}
	last := ec.last()
	if last.Name != "mcp:manager-closed" {
		t.Fatalf("Close() last event = %q, want %q", last.Name, "mcp:manager-closed")
	}
	if last.Payload != nil {
		t.Fatalf("Close() payload = %#v, want nil", last.Payload)
	}

	// Double close should not panic.
	mgr.Close()
	if ec.count() != 1 {
		t.Fatalf("second Close() should not emit new events, got count = %d", ec.count())
	}

	// SetEnabled after close should return error.
	err := mgr.SetEnabled("session-1", "memory", true)
	if err == nil {
		t.Error("SetEnabled after Close should return error")
	}
}

func TestManager_CloseWithoutEvent(t *testing.T) {
	mgr, ec := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	mgr.CloseWithoutEvent()
	if ec.count() != 0 {
		t.Fatalf("CloseWithoutEvent() emitted %d events, want 0", ec.count())
	}
}

func TestManager_CloseWithoutEvent_InvalidatesPendingStart(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	resolveEntered := make(chan struct{})
	releaseResolve := make(chan struct{})
	pipe := &fakeManagedPipeServer{
		pipeName:     `\\.\pipe\test-close-pending-start`,
		startEntered: make(chan struct{}),
		startRelease: make(chan struct{}),
	}

	mgr := NewManager(ManagerConfig{
		Registry: reg,
		EmitFn:   func(string, any) {},
		NewPipeServer: func(MCPPipeConfig) managedPipeServer {
			return pipe
		},
		ResolveWorkDir: func(string) (string, error) {
			close(resolveEntered)
			<-releaseResolve
			return ".", nil
		},
	})

	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	mustReceiveWithin(t, resolveEntered, time.Second, "ResolveWorkDir was not called")

	mgr.mu.RLock()
	inst := mgr.sessions["session-1"]["memory"]
	mgr.mu.RUnlock()
	if inst == nil {
		t.Fatal("session instance not found after SetEnabled")
	}

	mgr.CloseWithoutEvent()
	close(releaseResolve)

	mustNotReceiveWithin(t, pipe.startEntered, 200*time.Millisecond, "pipe Start should not be called after CloseWithoutEvent")

	inst.mu.RLock()
	defer inst.mu.RUnlock()
	if inst.generation != 2 {
		t.Fatalf("instance generation = %d, want 2 after invalidation", inst.generation)
	}
	if inst.cancel != nil {
		t.Fatal("instance cancel should be nil after CloseWithoutEvent")
	}
	if inst.pipe != nil {
		t.Fatal("instance pipe should be nil after CloseWithoutEvent")
	}
	if inst.state.Enabled {
		t.Fatal("instance should be disabled after CloseWithoutEvent")
	}
	if inst.state.Status != StatusStopped {
		t.Fatalf("instance status = %q, want %q", inst.state.Status, StatusStopped)
	}
}

func TestManager_CleanupSession_InvalidatesStartBlockedInPipeStart(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	pipe := &fakeManagedPipeServer{
		pipeName:     `\\.\pipe\test-cleanup-pending-start`,
		startEntered: make(chan struct{}),
		startRelease: make(chan struct{}),
	}

	mgr := NewManager(ManagerConfig{
		Registry: reg,
		EmitFn:   func(string, any) {},
		NewPipeServer: func(MCPPipeConfig) managedPipeServer {
			return pipe
		},
		ResolveWorkDir: func(string) (string, error) { return ".", nil },
	})

	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	mustReceiveWithin(t, pipe.startEntered, time.Second, "pipe Start was not called")

	mgr.mu.RLock()
	inst := mgr.sessions["session-1"]["memory"]
	mgr.mu.RUnlock()
	if inst == nil {
		t.Fatal("session instance not found after SetEnabled")
	}

	if err := mgr.CleanupSession("session-1"); err != nil {
		t.Fatalf("CleanupSession() error = %v", err)
	}
	close(pipe.startRelease)

	deadline := time.Now().Add(time.Second)
	for {
		_, stopCount := pipe.snapshot()
		if stopCount == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipe Stop count = %d, want 1", stopCount)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mgr.mu.RLock()
	_, stillPresent := mgr.sessions["session-1"]
	mgr.mu.RUnlock()
	if stillPresent {
		t.Fatal("session should be removed after CleanupSession")
	}

	inst.mu.RLock()
	defer inst.mu.RUnlock()
	if inst.generation != 2 {
		t.Fatalf("instance generation = %d, want 2 after CleanupSession invalidation", inst.generation)
	}
	if inst.cancel != nil {
		t.Fatal("instance cancel should be nil after CleanupSession")
	}
	if inst.pipe != nil {
		t.Fatal("instance pipe should be nil after CleanupSession")
	}
	if inst.state.Enabled {
		t.Fatal("instance should be disabled after CleanupSession")
	}
	if inst.state.Status != StatusStopped {
		t.Fatalf("instance status = %q, want %q", inst.state.Status, StatusStopped)
	}
}

func TestManager_SessionIsolation(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})

	// Enable in session-1, leave disabled in session-2.
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}

	snap1, err := mgr.SnapshotForSession("session-1")
	if err != nil {
		t.Fatalf("SnapshotForSession(session-1) failed: %v", err)
	}
	snap2, err := mgr.SnapshotForSession("session-2")
	if err != nil {
		t.Fatalf("SnapshotForSession(session-2) failed: %v", err)
	}

	if len(snap1) != 1 || !snap1[0].Enabled {
		t.Error("session-1 should have memory enabled")
	}
	if len(snap2) != 1 || snap2[0].Enabled {
		t.Error("session-2 should have memory disabled (session isolation)")
	}
}

func TestNewManager_NilRegistry(t *testing.T) {
	// Should not panic — uses empty registry.
	mgr := NewManager(ManagerConfig{EmitFn: func(string, any) {}})
	snapshots, err := mgr.SnapshotForSession("session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestNewManager_NilEmitFn(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(MCPDefinition{ID: "memory", Name: "Memory", Command: "test-command"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Should not panic — uses no-op emitter.
	mgr := NewManager(ManagerConfig{Registry: reg})
	if err := mgr.SetEnabled("session-1", "memory", true); err != nil {
		t.Fatalf("SetEnabled failed: %v", err)
	}
}

func TestStructFieldCounts(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"MCPDefinition", reflect.TypeFor[MCPDefinition]().NumField(), 10},
		{"MCPConfigParam", reflect.TypeFor[MCPConfigParam]().NumField(), 4},
		{"MCPInstanceState", reflect.TypeFor[MCPInstanceState]().NumField(), 5},
		{"MCPSnapshot", reflect.TypeFor[MCPSnapshot]().NumField(), 12},
		{"instance", reflect.TypeFor[instance]().NumField(), 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s field count = %d, want %d; update snapshot builders for new fields", tt.name, tt.got, tt.want)
			}
		})
	}
}
