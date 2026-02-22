package tmux

import (
	"fmt"
	"strings"
	"testing"

	"myT-x/internal/terminal"
)

func TestFallbackWindowIDNearIndex(t *testing.T) {
	tests := []struct {
		name         string
		windows      []*TmuxWindow
		preferredIdx int
		wantID       int
		wantOK       bool
	}{
		{
			name:         "empty windows returns false",
			windows:      []*TmuxWindow{},
			preferredIdx: 0,
			// wantID is ignored when wantOK=false (zero value is not meaningful).
			wantID: 0,
			wantOK: false,
		},
		{
			name:         "nil slice returns false",
			windows:      nil,
			preferredIdx: 0,
			// wantID is ignored when wantOK=false (zero value is not meaningful).
			wantID: 0,
			wantOK: false,
		},
		{
			name:         "single window returns its ID",
			windows:      []*TmuxWindow{{ID: 5, Name: "only"}},
			preferredIdx: 0,
			wantID:       5,
			wantOK:       true,
		},
		{
			name:         "preferred index at exact window",
			windows:      []*TmuxWindow{{ID: 10}, {ID: 20}, {ID: 30}},
			preferredIdx: 1,
			wantID:       20,
			wantOK:       true,
		},
		{
			name:         "preferred index beyond length clamps to last",
			windows:      []*TmuxWindow{{ID: 10}, {ID: 20}},
			preferredIdx: 99,
			wantID:       20,
			wantOK:       true,
		},
		{
			name:         "negative preferred index clamps to zero",
			windows:      []*TmuxWindow{{ID: 10}, {ID: 20}},
			preferredIdx: -5,
			wantID:       10,
			wantOK:       true,
		},
		{
			name:         "preferred index at nil window scans forward",
			windows:      []*TmuxWindow{nil, nil, {ID: 30}},
			preferredIdx: 0,
			wantID:       30,
			wantOK:       true,
		},
		{
			name:         "preferred index at nil window scans backward",
			windows:      []*TmuxWindow{{ID: 10}, nil, nil},
			preferredIdx: 2,
			wantID:       10,
			wantOK:       true,
		},
		{
			name:         "all nil windows returns false",
			windows:      []*TmuxWindow{nil, nil, nil},
			preferredIdx: 1,
			// wantID is ignored when wantOK=false (zero value is not meaningful).
			wantID: 0,
			wantOK: false,
		},
		{
			name:         "nil between valid windows prefers forward scan",
			windows:      []*TmuxWindow{{ID: 10}, nil, {ID: 30}},
			preferredIdx: 1,
			wantID:       30,
			wantOK:       true,
		},
		{
			name:         "deleted ID gap with forward fallback",
			windows:      []*TmuxWindow{{ID: 1}, {ID: 3}, {ID: 7}},
			preferredIdx: 1,
			wantID:       3,
			wantOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := fallbackWindowIDNearIndex(tt.windows, tt.preferredIdx)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Fatalf("fallbackWindowIDNearIndex(len=%d, idx=%d) = (%d, %v), want (%d, %v)",
					len(tt.windows), tt.preferredIdx, gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestRemoveWindowLastWindowRemovesSession(t *testing.T) {
	// S-33: Killing the last window should remove the entire session.
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	session, ok := manager.GetSession("demo")
	if !ok {
		t.Fatal("GetSession(demo) failed")
	}
	windowID := session.Windows[0].ID

	result, err := manager.RemoveWindowByID("demo", windowID)
	if err != nil {
		t.Fatalf("RemoveWindowByID() error = %v", err)
	}
	if !result.SessionRemoved {
		t.Fatal("SessionRemoved = false, want true (last window removed)")
	}
	if manager.HasSession("demo") {
		t.Fatal("session 'demo' should be removed after last window deletion")
	}
}

// --- I-07: findWindowIndexByID helper tests ---

func TestFindWindowIndexByID(t *testing.T) {
	tests := []struct {
		name    string
		windows []*TmuxWindow
		id      int
		want    int
	}{
		{
			name:    "nil slice returns -1",
			windows: nil,
			id:      1,
			want:    -1,
		},
		{
			name:    "empty slice returns -1",
			windows: []*TmuxWindow{},
			id:      1,
			want:    -1,
		},
		{
			name:    "not found returns -1",
			windows: []*TmuxWindow{{ID: 10}, {ID: 20}},
			id:      99,
			want:    -1,
		},
		{
			name:    "found at first position",
			windows: []*TmuxWindow{{ID: 10}, {ID: 20}, {ID: 30}},
			id:      10,
			want:    0,
		},
		{
			name:    "found at last position",
			windows: []*TmuxWindow{{ID: 10}, {ID: 20}, {ID: 30}},
			id:      30,
			want:    2,
		},
		{
			name:    "skips nil entries",
			windows: []*TmuxWindow{nil, {ID: 10}, nil, {ID: 20}},
			id:      20,
			want:    3,
		},
		{
			name:    "all nil returns -1",
			windows: []*TmuxWindow{nil, nil, nil},
			id:      1,
			want:    -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findWindowIndexByID(tt.windows, tt.id)
			if got != tt.want {
				t.Fatalf("findWindowIndexByID(len=%d, id=%d) = %d, want %d",
					len(tt.windows), tt.id, got, tt.want)
			}
		})
	}
}

// --- S-26 / T-8: RemoveWindowByID comprehensive tests ---

// T-8: Table-driven test for RemoveWindowByID covering 1-window model scenarios.
func TestRemoveWindowByID(t *testing.T) {
	tests := []struct {
		name                 string
		sessionName          string
		windowID             int                         // -1 means "use actual window ID from created session"
		setup                func(m *SessionManager) int // returns windowID to use, or -1 if windowID field is used
		wantErr              bool
		wantSessionGone      bool
		wantSurvivingID      int
		wantRemovedPanes     int
		wantPaneUnregistered bool // verify pane is removed from global map
	}{
		{
			name:                 "sole window removal deletes session",
			sessionName:          "test",
			windowID:             -1,
			wantErr:              false,
			wantSessionGone:      true,
			wantSurvivingID:      -1,
			wantRemovedPanes:     1,
			wantPaneUnregistered: true,
		},
		{
			name:        "non-existent session returns error",
			sessionName: "nonexistent",
			windowID:    0,
			wantErr:     true,
		},
		{
			name:        "non-existent window ID returns error",
			sessionName: "test",
			windowID:    99999,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSessionManager()

			var paneID int
			var windowID int

			// Create session unless testing non-existent session
			if tt.sessionName != "nonexistent" {
				_, pane, err := manager.CreateSession(tt.sessionName, "main", 120, 40)
				if err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
				paneID = pane.ID

				session, ok := manager.GetSession(tt.sessionName)
				if !ok {
					t.Fatal("GetSession() failed after CreateSession")
				}
				windowID = session.Windows[0].ID
			}

			if tt.setup != nil {
				if id := tt.setup(manager); id >= 0 {
					windowID = id
				}
			}

			targetWindowID := tt.windowID
			if targetWindowID == -1 {
				targetWindowID = windowID
			}

			// Verify pane is registered before removal
			if tt.wantPaneUnregistered {
				_, resolveErr := manager.ResolveTarget(fmt.Sprintf("%%%d", paneID), -1)
				if resolveErr != nil {
					t.Fatalf("pane %%%d should be resolvable before removal, got error: %v", paneID, resolveErr)
				}
			}

			result, err := manager.RemoveWindowByID(tt.sessionName, targetWindowID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RemoveWindowByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if result.SessionRemoved != tt.wantSessionGone {
				t.Fatalf("SessionRemoved = %v, want %v", result.SessionRemoved, tt.wantSessionGone)
			}
			if result.SurvivingWindowID != tt.wantSurvivingID {
				t.Fatalf("SurvivingWindowID = %d, want %d", result.SurvivingWindowID, tt.wantSurvivingID)
			}
			if len(result.RemovedPanes) != tt.wantRemovedPanes {
				t.Fatalf("len(RemovedPanes) = %d, want %d", len(result.RemovedPanes), tt.wantRemovedPanes)
			}

			// Verify session is gone from the manager
			if tt.wantSessionGone && manager.HasSession(tt.sessionName) {
				t.Fatal("session should be removed after last window deletion")
			}

			// Verify pane is unregistered from global pane map
			if tt.wantPaneUnregistered {
				_, resolveErr := manager.ResolveTarget(fmt.Sprintf("%%%d", paneID), -1)
				if resolveErr == nil {
					t.Fatalf("pane %%%d should be unresolvable after removal, but ResolveTarget succeeded", paneID)
				}
			}
		})
	}
}

// T-8b: RemovedPanes preserves Terminal reference for ConPTY cleanup.
// The caller (handleKillWindow) relies on Terminal being non-nil-ified
// in the returned panes to close ConPTY handles outside the lock.
func TestRemoveWindowByID_RemovedPanesPreserveTerminal(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	session, ok := manager.GetSession("test")
	if !ok {
		t.Fatal("GetSession() failed")
	}
	windowID := session.Windows[0].ID

	// Note: In unit tests without terminal attachment, Terminal is nil.
	// The critical contract is that the pane pointer itself is returned
	// (not a copy with Terminal stripped), so callers can check Terminal.
	result, removeErr := manager.RemoveWindowByID("test", windowID)
	if removeErr != nil {
		t.Fatalf("RemoveWindowByID() error = %v", removeErr)
	}

	if len(result.RemovedPanes) != 1 {
		t.Fatalf("len(RemovedPanes) = %d, want 1", len(result.RemovedPanes))
	}

	removedPane := result.RemovedPanes[0]
	if removedPane == nil {
		t.Fatal("RemovedPanes[0] is nil, want non-nil pane pointer")
	}
	if removedPane.ID != pane.ID {
		t.Fatalf("RemovedPanes[0].ID = %d, want %d", removedPane.ID, pane.ID)
	}
	// Terminal field must not be explicitly nil-ified by removeWindowAtIndexLocked.
	// (It is nil in this test because no terminal was attached, but the field
	// must be whatever the pane had — not forcibly cleared.)
}

// C-7: TestRemoveWindowByID_ActiveWindowTransitions verifies that removing the
// active window from a multi-window session transitions ActiveWindowID to a
// surviving window. Since AddWindow was removed (1-window model), multi-window
// state is constructed by directly manipulating internal fields under lock.
func TestRemoveWindowByID_ActiveWindowTransitions(t *testing.T) {
	manager := NewSessionManager()
	_, pane0, err := manager.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	session, ok := manager.GetSession("test")
	if !ok {
		t.Fatal("GetSession() failed")
	}
	activeWindowID := session.Windows[0].ID

	// Inject a second window to simulate multi-window state.
	secondWindow, secondPane := injectTestWindow(t, manager, "test", "second")

	// Remove the active window (the first one).
	result, removeErr := manager.RemoveWindowByID("test", activeWindowID)
	if removeErr != nil {
		t.Fatalf("RemoveWindowByID() error = %v", removeErr)
	}
	if result.SessionRemoved {
		t.Fatal("SessionRemoved = true, want false (second window survives)")
	}
	if result.SurvivingWindowID != secondWindow.ID {
		t.Fatalf("SurvivingWindowID = %d, want %d (second window)", result.SurvivingWindowID, secondWindow.ID)
	}

	// Verify session's ActiveWindowID was updated.
	afterSession, ok := manager.GetSession("test")
	if !ok {
		t.Fatal("GetSession() failed after removal")
	}
	if afterSession.ActiveWindowID != secondWindow.ID {
		t.Fatalf("ActiveWindowID = %d, want %d", afterSession.ActiveWindowID, secondWindow.ID)
	}
	if len(afterSession.Windows) != 1 {
		t.Fatalf("window count = %d, want 1", len(afterSession.Windows))
	}

	// Verify removed pane is unregistered from global map.
	_, resolveErr := manager.ResolveTarget(pane0.IDString(), -1)
	if resolveErr == nil {
		t.Fatal("removed pane should be unresolvable after window removal")
	}

	// Verify surviving pane is still resolvable.
	resolved, resolveErr := manager.ResolveTarget(secondPane.IDString(), -1)
	if resolveErr != nil {
		t.Fatalf("surviving pane should be resolvable, got error: %v", resolveErr)
	}
	if resolved.ID != secondPane.ID {
		t.Fatalf("resolved pane ID = %d, want %d", resolved.ID, secondPane.ID)
	}
}

// C-7b: TestRemoveWindowByID_NonActiveWindowKeepsActiveID verifies that removing
// a non-active window does not change the session's ActiveWindowID.
func TestRemoveWindowByID_NonActiveWindowKeepsActiveID(t *testing.T) {
	manager := NewSessionManager()
	_, _, err := manager.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Capture firstWindowID before injection.
	manager.mu.RLock()
	firstWindowID := manager.sessions["test"].Windows[0].ID
	manager.mu.RUnlock()

	// Inject a second window.
	secondWindow, _ := injectTestWindow(t, manager, "test", "second")

	// Remove the non-active (second) window.
	result, removeErr := manager.RemoveWindowByID("test", secondWindow.ID)
	if removeErr != nil {
		t.Fatalf("RemoveWindowByID() error = %v", removeErr)
	}
	if result.SessionRemoved {
		t.Fatal("SessionRemoved = true, want false")
	}
	// ActiveWindowID should remain the first window since we removed the non-active one.
	if result.SurvivingWindowID != firstWindowID {
		t.Fatalf("SurvivingWindowID = %d, want %d (first window unchanged)", result.SurvivingWindowID, firstWindowID)
	}
}

// C-8: TestRemoveWindowByID_ConcreteTerminalPreserved verifies that when a pane
// has a non-nil Terminal, the removed pane retains that Terminal reference in the
// result so the caller can perform ConPTY cleanup outside the lock.
func TestRemoveWindowByID_ConcreteTerminalPreserved(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("test", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Inject a concrete (non-nil) Terminal into the pane.
	// Using a zero-value Terminal struct: it is non-nil and provides a real
	// object for the ownership contract verification.
	concreteTerminal := &terminal.Terminal{}
	manager.mu.Lock()
	pane.Terminal = concreteTerminal
	manager.mu.Unlock()

	session, ok := manager.GetSession("test")
	if !ok {
		t.Fatal("GetSession() failed")
	}
	windowID := session.Windows[0].ID

	result, removeErr := manager.RemoveWindowByID("test", windowID)
	if removeErr != nil {
		t.Fatalf("RemoveWindowByID() error = %v", removeErr)
	}
	if len(result.RemovedPanes) != 1 {
		t.Fatalf("len(RemovedPanes) = %d, want 1", len(result.RemovedPanes))
	}

	removedPane := result.RemovedPanes[0]
	if removedPane == nil {
		t.Fatal("RemovedPanes[0] is nil, want non-nil pane pointer")
	}
	if removedPane.Terminal == nil {
		t.Fatal("RemovedPanes[0].Terminal is nil, want non-nil (concrete terminal should be preserved)")
	}
	if removedPane.Terminal != concreteTerminal {
		t.Fatal("RemovedPanes[0].Terminal is not the same pointer as the injected terminal")
	}
}

// --- SUG-07: RenameWindowByID comprehensive tests ---

func TestRenameWindowByID(t *testing.T) {
	tests := []struct {
		name            string
		sessionName     string
		windowID        int // -1 = use actual window ID from created session
		newName         string
		setup           func(m *SessionManager) int // returns windowID override, or -1
		wantErr         bool
		wantErrContains string
		wantWindowName  string
	}{
		{
			name:           "success renames window",
			sessionName:    "demo",
			windowID:       -1,
			newName:        "renamed",
			wantErr:        false,
			wantWindowName: "renamed",
		},
		{
			name:            "non-existent session returns error",
			sessionName:     "nonexistent",
			windowID:        0,
			newName:         "newname",
			wantErr:         true,
			wantErrContains: "not found",
		},
		{
			name:            "non-existent window ID returns error",
			sessionName:     "demo",
			windowID:        99999,
			newName:         "newname",
			wantErr:         true,
			wantErrContains: "window not found",
		},
		{
			name:            "empty new name returns error",
			sessionName:     "demo",
			windowID:        -1,
			newName:         "",
			wantErr:         true,
			wantErrContains: "cannot be empty",
		},
		{
			name:            "whitespace-only new name returns error",
			sessionName:     "demo",
			windowID:        -1,
			newName:         "   ",
			wantErr:         true,
			wantErrContains: "cannot be empty",
		},
		{
			name:           "trims whitespace from new name",
			sessionName:    "demo",
			windowID:       -1,
			newName:        "  trimmed  ",
			wantErr:        false,
			wantWindowName: "trimmed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSessionManager()

			var windowID int
			if tt.sessionName != "nonexistent" {
				_, _, err := manager.CreateSession(tt.sessionName, "main", 120, 40)
				if err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
				session, ok := manager.GetSession(tt.sessionName)
				if !ok {
					t.Fatal("GetSession() failed after CreateSession")
				}
				windowID = session.Windows[0].ID
			}

			if tt.setup != nil {
				if id := tt.setup(manager); id >= 0 {
					windowID = id
				}
			}

			targetWindowID := tt.windowID
			if targetWindowID == -1 {
				targetWindowID = windowID
			}

			idx, err := manager.RenameWindowByID(tt.sessionName, targetWindowID, tt.newName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RenameWindowByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("RenameWindowByID() error = %q, want containing %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			// Verify return index is valid (0-based)
			if idx < 0 {
				t.Fatalf("RenameWindowByID() returned negative index %d", idx)
			}

			// Verify the window was actually renamed
			session, ok := manager.GetSession(tt.sessionName)
			if !ok {
				t.Fatal("GetSession() failed after rename")
			}
			if len(session.Windows) == 0 {
				t.Fatal("no windows in session after rename")
			}
			if session.Windows[idx].Name != tt.wantWindowName {
				t.Fatalf("window name = %q, want %q", session.Windows[idx].Name, tt.wantWindowName)
			}
		})
	}
}
