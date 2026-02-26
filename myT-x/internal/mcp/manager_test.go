package mcp

import (
	"reflect"
	"strings"
	"sync"
	"testing"
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
	mgr := NewManager(reg, ec.emit)
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
	mgr.CleanupSession("session-1")

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
	mgr.CleanupSession("")
	mgr.CleanupSession("   ")
}

func TestManager_CleanupSession_NonExistingSession(t *testing.T) {
	mgr, _ := newTestManager(t, MCPDefinition{ID: "memory", Name: "Memory"})
	// Should not panic on missing sessions.
	mgr.CleanupSession("never-created")
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
	inst.cancel = func() { cancelCount++ }
	inst.mu.Unlock()

	mgr.CleanupSession("session-1")
	if cancelCount != 1 {
		t.Fatalf("CleanupSession() cancel count = %d, want 1", cancelCount)
	}
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
	inst.cancel = func() { cancelCount++ }
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
	mgr := NewManager(nil, func(string, any) {})
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
	mgr := NewManager(reg, nil)
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
		{"MCPDefinition", reflect.TypeFor[MCPDefinition]().NumField(), 9},
		{"MCPConfigParam", reflect.TypeFor[MCPConfigParam]().NumField(), 4},
		{"MCPInstanceState", reflect.TypeFor[MCPInstanceState]().NumField(), 5},
		{"MCPSnapshot", reflect.TypeFor[MCPSnapshot]().NumField(), 8},
		{"instance", reflect.TypeFor[instance]().NumField(), 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s field count = %d, want %d; update snapshot builders for new fields", tt.name, tt.got, tt.want)
			}
		})
	}
}
