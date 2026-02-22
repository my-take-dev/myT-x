package tmux

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResolveTargetByPaneID(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	resolved, err := manager.ResolveTarget("%0", -1)
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if resolved.ID != pane.ID {
		t.Fatalf("resolved pane id = %d, want %d", resolved.ID, pane.ID)
	}
}

// C-5: TestResolveTargetRepairsStaleActiveWindowID verifies that ResolveTarget
// handles a stale/invalid ActiveWindowID gracefully by auto-repairing to the
// first surviving window. The auto-repair path is triggered when the RLock fast
// path detects a stale ID and upgrades to a write Lock.
func TestResolveTargetRepairsStaleActiveWindowID(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Corrupt ActiveWindowID to a non-existent value.
	manager.mu.Lock()
	session := manager.sessions["demo"]
	originalWindowID := session.ActiveWindowID
	session.ActiveWindowID = 99999 // non-existent
	manager.mu.Unlock()

	// ResolveTarget with session name should trigger auto-repair and return
	// the active pane from the fallback window.
	resolved, resolveErr := manager.ResolveTarget("demo", -1)
	if resolveErr != nil {
		t.Fatalf("ResolveTarget(demo) error = %v, want nil (should auto-repair)", resolveErr)
	}
	if resolved == nil {
		t.Fatal("ResolveTarget(demo) returned nil pane")
	}
	if resolved.ID != pane.ID {
		t.Fatalf("resolved pane ID = %d, want %d (the only pane in the session)", resolved.ID, pane.ID)
	}

	// Verify the ActiveWindowID was repaired to the surviving window.
	manager.mu.RLock()
	repairedID := manager.sessions["demo"].ActiveWindowID
	manager.mu.RUnlock()
	if repairedID == 99999 {
		t.Fatal("ActiveWindowID was not repaired, still 99999")
	}
	if repairedID != originalWindowID {
		t.Fatalf("repaired ActiveWindowID = %d, want %d", repairedID, originalWindowID)
	}
}

// C-5b: TestResolveTargetWithEmptyTargetAndStaleActiveWindowID verifies that an
// empty target with callerPaneID=-1 also triggers the auto-repair path when the
// default session has a stale ActiveWindowID.
func TestResolveTargetWithEmptyTargetAndStaleActiveWindowID(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Corrupt ActiveWindowID.
	manager.mu.Lock()
	manager.sessions["demo"].ActiveWindowID = 88888
	manager.mu.Unlock()

	// Empty target with -1 callerPaneID falls through to defaultPane path.
	resolved, resolveErr := manager.ResolveTarget("", -1)
	if resolveErr != nil {
		t.Fatalf("ResolveTarget(\"\", -1) error = %v, want nil (should auto-repair)", resolveErr)
	}
	if resolved == nil {
		t.Fatal("ResolveTarget returned nil pane")
	}
	if resolved.ID != pane.ID {
		t.Fatalf("resolved pane ID = %d, want %d", resolved.ID, pane.ID)
	}
}

func TestResolveSessionTargetReturnsClone(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	resolved, err := manager.ResolveSessionTarget("demo")
	if err != nil {
		t.Fatalf("ResolveSessionTarget() error = %v", err)
	}
	resolved.Name = "mutated"
	if len(resolved.Windows) == 0 {
		t.Fatal("resolved session has no windows")
	}
	resolved.Windows[0].Name = "mutated-window"

	current, ok := manager.GetSession("demo")
	if !ok {
		t.Fatal("GetSession(demo) returned missing session")
	}
	if current.Name != "demo" {
		t.Fatalf("session name mutated through ResolveSessionTarget clone: %q", current.Name)
	}
	if current.Windows[0].Name != "main" {
		t.Fatalf("window name mutated through ResolveSessionTarget clone: %q", current.Windows[0].Name)
	}
}

func TestSplitPaneUpdatesLayout(t *testing.T) {
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	newPane, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}
	if newPane.ID != 1 {
		t.Fatalf("new pane id = %d, want 1", newPane.ID)
	}
	if session.Windows[0].Layout == nil || session.Windows[0].Layout.Type != LayoutSplit {
		t.Fatalf("layout not split: %#v", session.Windows[0].Layout)
	}
}

func TestRenamePane(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	sessionName, err := manager.RenamePane(pane.IDString(), "editor")
	if err != nil {
		t.Fatalf("RenamePane() error = %v", err)
	}
	if sessionName != "demo" {
		t.Fatalf("session name = %q, want demo", sessionName)
	}
	resolved, err := manager.ResolveTarget(pane.IDString(), -1)
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if resolved.Title != "editor" {
		t.Fatalf("pane title = %q, want editor", resolved.Title)
	}
}

func TestSwapPanes(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	second, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}
	pane.Title = "left"
	second.Title = "right"

	sessionName, err := manager.SwapPanes(pane.IDString(), second.IDString())
	if err != nil {
		t.Fatalf("SwapPanes() error = %v", err)
	}
	if sessionName != "demo" {
		t.Fatalf("session name = %q, want demo", sessionName)
	}

	sessions := manager.Snapshot()
	if len(sessions) != 1 || len(sessions[0].Windows) != 1 || len(sessions[0].Windows[0].Panes) != 2 {
		t.Fatalf("unexpected snapshot shape: %#v", sessions)
	}
	panes := sessions[0].Windows[0].Panes
	if panes[0].ID != second.IDString() || panes[1].ID != pane.IDString() {
		t.Fatalf("pane order did not swap: %#v", panes)
	}
	layout := sessions[0].Windows[0].Layout
	if layout == nil || layout.Children[0] == nil || layout.Children[1] == nil {
		t.Fatalf("layout missing children after swap: %#v", layout)
	}
	if layout.Children[0].PaneID != second.ID || layout.Children[1].PaneID != pane.ID {
		t.Fatalf("layout pane ids not swapped: %#v", layout)
	}
}

func TestSwapPanesRebuildsLayoutWhenLayoutMissing(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	second, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	manager.mu.Lock()
	manager.sessions["demo"].Windows[0].Layout = nil
	manager.mu.Unlock()

	if _, err := manager.SwapPanes(pane.IDString(), second.IDString()); err != nil {
		t.Fatalf("SwapPanes() error = %v", err)
	}

	snapshots := manager.Snapshot()
	layout := snapshots[0].Windows[0].Layout
	if layout == nil {
		t.Fatal("layout is nil after SwapPanes fallback rebuild")
	}
	if layout.Type != LayoutSplit {
		t.Fatalf("layout type = %q, want %q", layout.Type, LayoutSplit)
	}
}

func TestSnapshotPreservesPaneIDZero(t *testing.T) {
	manager := NewSessionManager()
	_, _, err := manager.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	snapshots := manager.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 session, got %d", len(snapshots))
	}
	layout := snapshots[0].Windows[0].Layout
	if layout == nil {
		t.Fatal("layout is nil")
	}
	if layout.Type != LayoutLeaf {
		t.Fatalf("layout type = %q, want %q", layout.Type, LayoutLeaf)
	}
	if layout.PaneID != 0 {
		t.Fatalf("layout PaneID = %d, want 0", layout.PaneID)
	}

	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	if !strings.Contains(string(data), `"pane_id":0`) {
		t.Fatalf("JSON does not contain pane_id:0: %s", string(data))
	}
}

func TestKillPanePreservesMixedLayoutDirection(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	rightPane, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane(horizontal) error = %v", err)
	}
	bottomPane, err := manager.SplitPane(pane.ID, SplitVertical)
	if err != nil {
		t.Fatalf("SplitPane(vertical) error = %v", err)
	}

	_, removedSession, err := manager.KillPane(rightPane.IDString())
	if err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	if removedSession {
		t.Fatal("removedSession = true, want false")
	}

	snapshots := manager.Snapshot()
	if len(snapshots) != 1 || len(snapshots[0].Windows) != 1 {
		t.Fatalf("unexpected snapshot shape: %#v", snapshots)
	}
	window := snapshots[0].Windows[0]
	if len(window.Panes) != 2 {
		t.Fatalf("pane count = %d, want 2", len(window.Panes))
	}
	layout := window.Layout
	if layout == nil {
		t.Fatal("layout is nil")
	}
	if layout.Type != LayoutSplit {
		t.Fatalf("layout type = %q, want %q", layout.Type, LayoutSplit)
	}
	if layout.Direction != SplitVertical {
		t.Fatalf("layout direction = %q, want %q", layout.Direction, SplitVertical)
	}
	if layout.Children[0] == nil || layout.Children[1] == nil {
		t.Fatalf("layout children missing: %#v", layout)
	}
	if layout.Children[0].Type != LayoutLeaf || layout.Children[1].Type != LayoutLeaf {
		t.Fatalf("children are not both leaf nodes: %#v", layout)
	}

	gotIDs := map[int]bool{
		layout.Children[0].PaneID: true,
		layout.Children[1].PaneID: true,
	}
	if !gotIDs[pane.ID] || !gotIDs[bottomPane.ID] {
		t.Fatalf("layout leaves = %#v, want pane %d and %d", gotIDs, pane.ID, bottomPane.ID)
	}
}

func TestSessionIdleStateTransitions(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	manager := NewSessionManager()
	manager.now = func() time.Time { return now }
	manager.idleThreshold = 5 * time.Second

	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	snapshots := manager.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	if snapshots[0].IsIdle {
		t.Fatal("new session should be active")
	}

	now = now.Add(6 * time.Second)
	if changed := manager.CheckIdleState(); !changed {
		t.Fatal("CheckIdleState() should report idle transition")
	}

	snapshots = manager.Snapshot()
	if !snapshots[0].IsIdle {
		t.Fatal("session should be idle after threshold")
	}

	if changed := manager.CheckIdleState(); changed {
		t.Fatal("CheckIdleState() should not report change without transition")
	}

	now = now.Add(1 * time.Second)
	if changed := manager.UpdateActivityByPaneID(pane.IDString()); !changed {
		t.Fatal("UpdateActivityByPaneID() should report idle-to-active transition")
	}

	snapshots = manager.Snapshot()
	if snapshots[0].IsIdle {
		t.Fatal("session should return to active after output")
	}
}

func TestUpdateActivityByPaneIDUpdatesTimestamp(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	manager := NewSessionManager()
	manager.now = func() time.Time { return now }

	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	now = now.Add(2 * time.Second)
	_ = manager.UpdateActivityByPaneID(pane.IDString())

	if !session.LastActivity.Equal(now) {
		t.Fatalf("LastActivity = %s, want %s", session.LastActivity, now)
	}
}

func TestGetSessionEnvReturnsCopy(t *testing.T) {
	manager := NewSessionManager()
	session, _, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session.Env["FOO"] = "bar"

	env, err := manager.GetSessionEnv("demo")
	if err != nil {
		t.Fatalf("GetSessionEnv() error = %v", err)
	}
	if env["FOO"] != "bar" {
		t.Fatalf("env[FOO] = %q, want bar", env["FOO"])
	}
	env["FOO"] = "changed"
	if session.Env["FOO"] != "bar" {
		t.Fatalf("session env should not be mutated, got %q", session.Env["FOO"])
	}
}

func TestGetSessionEnvErrors(t *testing.T) {
	manager := NewSessionManager()
	_, _, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty name", "", "session name is required"},
		{"whitespace only", "   ", "session name is required"},
		{"not found", "nonexistent", "session not found: nonexistent"},
		{"colon-stripped resolves", "demo:0", ""}, // should succeed
		{"colon-stripped not found", "nope:0", "session not found: nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.GetSessionEnv(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetPaneEnvErrors(t *testing.T) {
	manager := NewSessionManager()
	_, _, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"invalid format no percent", "0", "invalid pane id"},
		{"empty string", "", "invalid pane id"},
		{"not found", "%999", "pane not found: %999"},
		{"valid pane", "%0", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.GetPaneEnv(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetPaneEnvReturnsCopy(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane.Env["FOO"] = "bar"

	env, err := manager.GetPaneEnv(pane.IDString())
	if err != nil {
		t.Fatalf("GetPaneEnv() error = %v", err)
	}
	if env["FOO"] != "bar" {
		t.Fatalf("env[FOO] = %q, want bar", env["FOO"])
	}
	env["FOO"] = "changed"
	if pane.Env["FOO"] != "bar" {
		t.Fatalf("pane env should not be mutated, got %q", pane.Env["FOO"])
	}
}

func TestSnapshotIsAgentTeamPropagation(t *testing.T) {
	tests := []struct {
		name        string
		isAgentTeam bool
	}{
		{"normal session", false},
		{"agent team session", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSessionManager()
			session, _, err := manager.CreateSession("test", "main", 120, 40)
			if err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
			session.IsAgentTeam = tt.isAgentTeam

			snapshots := manager.Snapshot()
			if len(snapshots) != 1 {
				t.Fatalf("expected 1 session, got %d", len(snapshots))
			}
			if snapshots[0].IsAgentTeam != tt.isAgentTeam {
				t.Fatalf("IsAgentTeam = %v, want %v", snapshots[0].IsAgentTeam, tt.isAgentTeam)
			}
		})
	}
}

func TestListSessionsReturnsIndependentCopies(t *testing.T) {
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session.Env["ROLE"] = "lead"
	pane.Title = "original-pane"

	listed := manager.ListSessions()
	if len(listed) != 1 {
		t.Fatalf("ListSessions() length = %d, want 1", len(listed))
	}
	listed[0].Name = "changed-session"
	listed[0].Env["ROLE"] = "mutated"
	listed[0].Windows[0].Panes[0].Title = "changed-pane"

	fresh := manager.Snapshot()
	if fresh[0].Name != "demo" {
		t.Fatalf("snapshot session name = %q, want %q", fresh[0].Name, "demo")
	}
	if freshPaneTitle := fresh[0].Windows[0].Panes[0].Title; freshPaneTitle != "original-pane" {
		t.Fatalf("snapshot pane title = %q, want %q", freshPaneTitle, "original-pane")
	}
}

func TestGetSessionReturnsIndependentCopy(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane.Title = "original-pane"

	got, ok := manager.GetSession("demo:0")
	if !ok || got == nil {
		t.Fatal("GetSession(demo:0) should return a session copy")
	}
	got.Name = "changed-session"
	got.Windows[0].Name = "changed-window"
	got.Windows[0].Panes[0].Title = "changed-pane"

	fresh, ok := manager.GetSession("demo")
	if !ok || fresh == nil {
		t.Fatal("GetSession(demo) should return a session")
	}
	if fresh.Name != "demo" {
		t.Fatalf("fresh session name = %q, want %q", fresh.Name, "demo")
	}
	if fresh.Windows[0].Name != "main" {
		t.Fatalf("fresh window name = %q, want %q", fresh.Windows[0].Name, "main")
	}
	if freshPaneTitle := fresh.Windows[0].Panes[0].Title; freshPaneTitle != "original-pane" {
		t.Fatalf("fresh pane title = %q, want %q", freshPaneTitle, "original-pane")
	}
}

func TestRemoveSessionReturnsDetachedCopy(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane.Title = "original-pane"

	removed, err := manager.RemoveSession("demo")
	if err != nil {
		t.Fatalf("RemoveSession() error = %v", err)
	}
	if removed == nil {
		t.Fatal("RemoveSession() returned nil session")
	}
	if manager.HasSession("demo") {
		t.Fatal("session should be removed from manager")
	}

	removed.Name = "mutated-after-remove"
	removed.Windows[0].Panes[0].Title = "mutated-pane"
	if removed.Windows[0].Panes[0].Title != "mutated-pane" {
		t.Fatal("returned copy should remain mutable for callers")
	}
}

func TestKillPaneRemovesSessionWhenLastPane(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	sessionName, removedSession, err := manager.KillPane(pane.IDString())
	if err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	if sessionName != "demo" {
		t.Fatalf("sessionName = %q, want %q", sessionName, "demo")
	}
	if !removedSession {
		t.Fatal("removedSession = false, want true")
	}
	if manager.HasSession("demo") {
		t.Fatal("session should be removed")
	}

	// Verify pane is removed from internal map.
	manager.mu.RLock()
	_, paneExists := manager.panes[pane.ID]
	paneCount := len(manager.panes)
	manager.mu.RUnlock()
	if paneExists {
		t.Fatal("pane should be removed from internal panes map")
	}
	if paneCount != 0 {
		t.Fatalf("pane count = %d, want 0", paneCount)
	}
}

func TestKillPaneOrphanedPaneCleanup(t *testing.T) {
	// I-11: When session is deleted via KillPane, orphaned panes
	// that exist in m.panes but are not in any window's Panes slice
	// should be defensively cleaned up.
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Inject an orphaned pane into m.panes that references the same
	// session's window but is NOT in the window's Panes slice.
	orphanPane := &TmuxPane{
		ID:     999,
		Index:  0,
		Window: session.Windows[0],
		Env:    map[string]string{},
	}
	manager.mu.Lock()
	manager.panes[999] = orphanPane
	manager.mu.Unlock()

	// Kill the only visible pane -> session should be deleted.
	_, removedSession, err := manager.KillPane(pane.IDString())
	if err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	if !removedSession {
		t.Fatal("removedSession = false, want true")
	}

	// Verify the orphaned pane was also cleaned up.
	manager.mu.RLock()
	_, orphanExists := manager.panes[999]
	paneCount := len(manager.panes)
	manager.mu.RUnlock()
	if orphanExists {
		t.Fatal("orphaned pane should be cleaned up during session deletion")
	}
	if paneCount != 0 {
		t.Fatalf("pane count = %d, want 0 (orphan not cleaned up)", paneCount)
	}
}

func TestRemoveSessionCleansPanesMap(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane2, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	manager.mu.RLock()
	beforeCount := len(manager.panes)
	manager.mu.RUnlock()
	if beforeCount != 2 {
		t.Fatalf("pane count before = %d, want 2", beforeCount)
	}

	_, err = manager.RemoveSession("demo")
	if err != nil {
		t.Fatalf("RemoveSession() error = %v", err)
	}

	manager.mu.RLock()
	_, p1Exists := manager.panes[pane.ID]
	_, p2Exists := manager.panes[pane2.ID]
	afterCount := len(manager.panes)
	manager.mu.RUnlock()
	if p1Exists || p2Exists {
		t.Fatal("panes should be removed from internal map after RemoveSession")
	}
	if afterCount != 0 {
		t.Fatalf("pane count after = %d, want 0", afterCount)
	}
}

func TestCloseCleansPanesAndSessions(t *testing.T) {
	manager := NewSessionManager()
	_, _, err := manager.CreateSession("s1", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(s1) error = %v", err)
	}
	_, _, err = manager.CreateSession("s2", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(s2) error = %v", err)
	}

	manager.Close()

	manager.mu.RLock()
	sessionCount := len(manager.sessions)
	paneCount := len(manager.panes)
	manager.mu.RUnlock()
	if sessionCount != 0 {
		t.Fatalf("session count = %d, want 0", sessionCount)
	}
	if paneCount != 0 {
		t.Fatalf("pane count = %d, want 0", paneCount)
	}
}

func TestKillPaneKeepsOtherPanesInSession(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane2, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	sessionName, removedSession, err := manager.KillPane(pane.IDString())
	if err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	if sessionName != "demo" {
		t.Fatalf("sessionName = %q, want %q", sessionName, "demo")
	}
	if removedSession {
		t.Fatal("removedSession = true, want false (other pane still exists)")
	}

	// Verify pane2 is still in the panes map.
	manager.mu.RLock()
	_, p1Exists := manager.panes[pane.ID]
	_, p2Exists := manager.panes[pane2.ID]
	paneCount := len(manager.panes)
	manager.mu.RUnlock()
	if p1Exists {
		t.Fatal("killed pane should be removed from panes map")
	}
	if !p2Exists {
		t.Fatal("remaining pane should still be in panes map")
	}
	if paneCount != 1 {
		t.Fatalf("pane count = %d, want 1", paneCount)
	}
}

func TestKillPaneRepairsActiveWindowIDWithNilWindowEntries(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	secondPane, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	manager.mu.Lock()
	session := manager.sessions["demo"]
	session.Windows = []*TmuxWindow{nil, session.Windows[0]}
	session.ActiveWindowID = -999
	manager.mu.Unlock()

	_, removedSession, err := manager.KillPane(secondPane.IDString())
	if err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	if removedSession {
		t.Fatal("removedSession = true, want false")
	}

	snapshot, ok := manager.GetSession("demo")
	if !ok {
		t.Fatal("GetSession(demo) failed")
	}
	if len(snapshot.Windows) != 1 {
		t.Fatalf("window count = %d, want 1", len(snapshot.Windows))
	}
	if snapshot.ActiveWindowID != snapshot.Windows[0].ID {
		t.Fatalf("ActiveWindowID = %d, want %d", snapshot.ActiveWindowID, snapshot.Windows[0].ID)
	}
}

func TestTopologyGenerationTracksStructuralChanges(t *testing.T) {
	manager := NewSessionManager()
	initialTopology := manager.TopologyGeneration()

	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	afterCreate := manager.TopologyGeneration()
	if afterCreate <= initialTopology {
		t.Fatalf("topology generation after create = %d, want > %d", afterCreate, initialTopology)
	}

	if err := manager.SetActivePane(pane.ID); err != nil {
		t.Fatalf("SetActivePane() error = %v", err)
	}
	afterFocus := manager.TopologyGeneration()
	if afterFocus <= afterCreate {
		t.Fatalf("topology generation after focus = %d, want > %d", afterFocus, afterCreate)
	}

	splitPane, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}
	afterSplit := manager.TopologyGeneration()
	if afterSplit <= afterFocus {
		t.Fatalf("topology generation after split = %d, want > %d", afterSplit, afterFocus)
	}

	if _, err := manager.RenamePane(splitPane.IDString(), "renamed"); err != nil {
		t.Fatalf("RenamePane() error = %v", err)
	}
	if got := manager.TopologyGeneration(); got != afterSplit {
		t.Fatalf("RenamePane should not change topology generation: got %d want %d", got, afterSplit)
	}

	if _, _, err := manager.KillPane(splitPane.IDString()); err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	afterKill := manager.TopologyGeneration()
	if afterKill <= afterSplit {
		t.Fatalf("topology generation after kill = %d, want > %d", afterKill, afterSplit)
	}
}

func TestSnapshotCacheInvalidationByGeneration(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	first := manager.Snapshot()
	second := manager.Snapshot()
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("unexpected snapshot lengths: first=%d second=%d", len(first), len(second))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("snapshot should remain unchanged without mutations: first=%+v second=%+v", first, second)
	}
	manager.mu.RLock()
	beforeMutationSnapshotGen := manager.snapshotGeneration
	manager.mu.RUnlock()

	if _, err := manager.RenamePane(pane.IDString(), "changed"); err != nil {
		t.Fatalf("RenamePane() error = %v", err)
	}
	third := manager.Snapshot()
	if len(third) == 0 {
		t.Fatal("third snapshot should not be empty")
	}
	if got := third[0].Windows[0].Panes[0].Title; got != "changed" {
		t.Fatalf("snapshot title = %q, want %q after mutation", got, "changed")
	}
	manager.mu.RLock()
	afterMutationSnapshotGen := manager.snapshotGeneration
	currentGeneration := manager.generation
	manager.mu.RUnlock()
	if afterMutationSnapshotGen <= beforeMutationSnapshotGen {
		t.Fatalf("snapshotGeneration did not advance after mutation: before=%d after=%d",
			beforeMutationSnapshotGen, afterMutationSnapshotGen)
	}
	if afterMutationSnapshotGen != currentGeneration {
		t.Fatalf("snapshotGeneration should match current generation after rebuild: snapshot=%d generation=%d",
			afterMutationSnapshotGen, currentGeneration)
	}
}

func TestSnapshotCacheHitReturnsIndependentCopies(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := manager.RenamePane(pane.IDString(), "original-pane"); err != nil {
		t.Fatalf("RenamePane() error = %v", err)
	}
	if err := manager.SetWorktreeInfo("demo", &SessionWorktreeInfo{
		Path:       `C:\repo.wt\demo`,
		RepoPath:   `C:\repo`,
		BranchName: "demo",
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	first := manager.Snapshot()
	if len(first) != 1 || len(first[0].Windows) != 1 || len(first[0].Windows[0].Panes) != 1 {
		t.Fatalf("unexpected first snapshot shape: %+v", first)
	}

	first[0].Name = "mutated-session"
	first[0].Windows[0].Name = "mutated-window"
	first[0].Windows[0].Panes[0].Title = "mutated-pane"
	if first[0].Worktree != nil {
		first[0].Worktree.Path = `C:\mutated`
	}
	if first[0].Windows[0].Layout != nil {
		first[0].Windows[0].Layout.PaneID = 999
	}

	second := manager.Snapshot()
	if len(second) != 1 || len(second[0].Windows) != 1 || len(second[0].Windows[0].Panes) != 1 {
		t.Fatalf("unexpected second snapshot shape: %+v", second)
	}
	if second[0].Name != "demo" {
		t.Fatalf("snapshot session name = %q, want %q", second[0].Name, "demo")
	}
	if second[0].Windows[0].Name != "main" {
		t.Fatalf("snapshot window name = %q, want %q", second[0].Windows[0].Name, "main")
	}
	if second[0].Windows[0].Panes[0].Title != "original-pane" {
		t.Fatalf("snapshot pane title = %q, want %q", second[0].Windows[0].Panes[0].Title, "original-pane")
	}
	if second[0].Worktree == nil || second[0].Worktree.Path != `C:\repo.wt\demo` {
		t.Fatalf("snapshot worktree path = %+v, want %q", second[0].Worktree, `C:\repo.wt\demo`)
	}
	if second[0].Windows[0].Layout != nil && second[0].Windows[0].Layout.PaneID != pane.ID {
		t.Fatalf("snapshot layout pane id = %d, want %d", second[0].Windows[0].Layout.PaneID, pane.ID)
	}
}

func TestCloneSessionSnapshotsIndependence(t *testing.T) {
	// S-31: Verify that cloneSessionSnapshots produces a deep copy,
	// meaning mutations to the clone do not affect the original.
	src := []SessionSnapshot{
		{
			ID:             1,
			Name:           "original-session",
			ActiveWindowID: 0,
			IsAgentTeam:    true,
			RootPath:       `C:\original`,
			Worktree: &SessionWorktreeInfo{
				Path:       `C:\wt`,
				RepoPath:   `C:\repo`,
				BranchName: "main",
				BaseBranch: "develop",
				IsDetached: false,
			},
			Windows: []WindowSnapshot{
				{
					ID:       0,
					Name:     "original-window",
					ActivePN: 0,
					Layout:   &LayoutNode{Type: LayoutLeaf, PaneID: 10},
					Panes: []PaneSnapshot{
						{ID: "%10", Index: 0, Title: "original-pane", Active: true, Width: 120, Height: 40},
						{ID: "%11", Index: 1, Title: "second-pane", Active: false, Width: 80, Height: 24},
					},
				},
			},
		},
	}

	cloned := cloneSessionSnapshots(src)

	// Mutate every field of the clone.
	cloned[0].Name = "mutated-session"
	cloned[0].ActiveWindowID = 999
	cloned[0].IsAgentTeam = false
	cloned[0].RootPath = `C:\mutated`
	cloned[0].Worktree.Path = `C:\mutated-wt`
	cloned[0].Worktree.BranchName = "mutated-branch"
	cloned[0].Worktree.IsDetached = true
	cloned[0].Windows[0].Name = "mutated-window"
	cloned[0].Windows[0].ActivePN = 99
	cloned[0].Windows[0].Layout.PaneID = 999
	cloned[0].Windows[0].Panes[0].Title = "mutated-pane"
	cloned[0].Windows[0].Panes[0].Width = 1

	// Verify original is unchanged.
	if src[0].Name != "original-session" {
		t.Fatalf("src session name mutated: %q", src[0].Name)
	}
	if src[0].ActiveWindowID != 0 {
		t.Fatalf("src ActiveWindowID mutated: %d", src[0].ActiveWindowID)
	}
	if !src[0].IsAgentTeam {
		t.Fatal("src IsAgentTeam mutated to false")
	}
	if src[0].RootPath != `C:\original` {
		t.Fatalf("src RootPath mutated: %q", src[0].RootPath)
	}
	if src[0].Worktree.Path != `C:\wt` {
		t.Fatalf("src Worktree.Path mutated: %q", src[0].Worktree.Path)
	}
	if src[0].Worktree.BranchName != "main" {
		t.Fatalf("src Worktree.BranchName mutated: %q", src[0].Worktree.BranchName)
	}
	if src[0].Worktree.IsDetached {
		t.Fatal("src Worktree.IsDetached mutated to true")
	}
	if src[0].Windows[0].Name != "original-window" {
		t.Fatalf("src window name mutated: %q", src[0].Windows[0].Name)
	}
	if src[0].Windows[0].ActivePN != 0 {
		t.Fatalf("src ActivePN mutated: %d", src[0].Windows[0].ActivePN)
	}
	if src[0].Windows[0].Layout.PaneID != 10 {
		t.Fatalf("src layout PaneID mutated: %d", src[0].Windows[0].Layout.PaneID)
	}
	if src[0].Windows[0].Panes[0].Title != "original-pane" {
		t.Fatalf("src pane title mutated: %q", src[0].Windows[0].Panes[0].Title)
	}
	if src[0].Windows[0].Panes[0].Width != 120 {
		t.Fatalf("src pane width mutated: %d", src[0].Windows[0].Panes[0].Width)
	}
}

func TestCloneSessionSnapshotsEmptyInput(t *testing.T) {
	cloned := cloneSessionSnapshots([]SessionSnapshot{})
	if cloned == nil {
		t.Fatal("cloneSessionSnapshots(empty) should return non-nil empty slice")
	}
	if len(cloned) != 0 {
		t.Fatalf("cloneSessionSnapshots(empty) length = %d, want 0", len(cloned))
	}
}

func TestCloneSessionSnapshotsNilWorktree(t *testing.T) {
	src := []SessionSnapshot{
		{
			ID:       1,
			Name:     "no-worktree",
			Worktree: nil,
			Windows:  []WindowSnapshot{},
		},
	}

	cloned := cloneSessionSnapshots(src)
	if cloned[0].Worktree != nil {
		t.Fatal("cloned worktree should be nil when source is nil")
	}
}

func TestCreateSessionDuplicateNameReturnsError(t *testing.T) {
	manager := NewSessionManager()
	_, _, err := manager.CreateSession("duplicate", "main", 120, 40)
	if err != nil {
		t.Fatalf("first CreateSession() error = %v", err)
	}

	_, _, err = manager.CreateSession("duplicate", "main", 120, 40)
	if err == nil {
		t.Fatal("second CreateSession() with same name should return error")
	}
	if !strings.Contains(err.Error(), "session already exists: duplicate") {
		t.Fatalf("error = %q, want containing %q", err.Error(), "session already exists: duplicate")
	}

	// Original session should remain intact.
	session, ok := manager.GetSession("duplicate")
	if !ok {
		t.Fatal("GetSession(duplicate) should still find the original session")
	}
	if session.Name != "duplicate" {
		t.Fatalf("session name = %q, want %q", session.Name, "duplicate")
	}
}

func TestCreateSessionEmptyNameGetsAutoName(t *testing.T) {
	manager := NewSessionManager()

	session, _, err := manager.CreateSession("", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(\"\") error = %v", err)
	}
	if session.Name == "" {
		t.Fatal("auto-generated session name should not be empty")
	}
}

func TestCreateSessionWhitespaceOnlyNameGetsAutoName(t *testing.T) {
	manager := NewSessionManager()

	session, _, err := manager.CreateSession("   ", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(whitespace) error = %v", err)
	}
	if session.Name == "" || strings.TrimSpace(session.Name) == "" {
		t.Fatal("auto-generated session name should not be blank")
	}
}

func TestCreateSessionZeroDimensionsDefaultToSane(t *testing.T) {
	manager := NewSessionManager()

	_, pane, err := manager.CreateSession("demo", "main", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession(0x0) error = %v", err)
	}
	if pane.Width <= 0 || pane.Height <= 0 {
		t.Fatalf("zero dimensions should default to positive values: %dx%d", pane.Width, pane.Height)
	}
}

func TestCreateSessionNegativeDimensionsDefaultToSane(t *testing.T) {
	manager := NewSessionManager()

	_, pane, err := manager.CreateSession("demo", "main", -10, -20)
	if err != nil {
		t.Fatalf("CreateSession(negative) error = %v", err)
	}
	if pane.Width <= 0 || pane.Height <= 0 {
		t.Fatalf("negative dimensions should default to positive values: %dx%d", pane.Width, pane.Height)
	}
}
