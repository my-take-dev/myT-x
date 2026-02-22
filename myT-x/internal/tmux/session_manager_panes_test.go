package tmux

import (
	"strings"
	"testing"
)

// C-06: ApplyLayoutPresetByWindowID tests covering:
// - Normal: valid window ID applies layout preset
// - Error: non-existent session name
// - Error: non-existent window ID in session
// - Error: window with no panes (all nil pane entries)
// - Normal: multiple presets applied sequentially
func TestApplyLayoutPresetByWindowID(t *testing.T) {
	t.Run("valid window ID applies layout", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		manager.mu.RLock()
		windowID := pane0.Window.ID
		manager.mu.RUnlock()

		err = manager.ApplyLayoutPresetByWindowID("demo", windowID, PresetEvenHorizontal)
		if err != nil {
			t.Fatalf("ApplyLayoutPresetByWindowID() error = %v", err)
		}

		// Verify the layout was rebuilt.
		snapshots := manager.Snapshot()
		if len(snapshots) != 1 || len(snapshots[0].Windows) != 1 {
			t.Fatalf("unexpected snapshot shape: sessions=%d", len(snapshots))
		}
		layout := snapshots[0].Windows[0].Layout
		if layout == nil {
			t.Fatal("layout is nil after ApplyLayoutPresetByWindowID")
		}
		if layout.Type != LayoutSplit {
			t.Fatalf("layout type = %q, want %q", layout.Type, LayoutSplit)
		}
		_ = pane1 // ensure second pane exists
	})

	t.Run("non-existent session returns error", func(t *testing.T) {
		manager := NewSessionManager()

		err := manager.ApplyLayoutPresetByWindowID("nonexistent", 0, PresetEvenHorizontal)
		if err == nil {
			t.Fatal("expected error for non-existent session, got nil")
		}
		if !strings.Contains(err.Error(), "session not found") {
			t.Fatalf("error = %q, want containing 'session not found'", err.Error())
		}
	})

	t.Run("non-existent window ID returns error", func(t *testing.T) {
		manager := NewSessionManager()
		if _, _, err := manager.CreateSession("demo", "main", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		err := manager.ApplyLayoutPresetByWindowID("demo", 9999, PresetEvenHorizontal)
		if err == nil {
			t.Fatal("expected error for non-existent window ID, got nil")
		}
		if !strings.Contains(err.Error(), "window not found") {
			t.Fatalf("error = %q, want containing 'window not found'", err.Error())
		}
	})

	t.Run("window with no panes returns error", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		manager.mu.RLock()
		windowID := pane.Window.ID
		manager.mu.RUnlock()

		// Clear all panes from window to simulate empty pane list.
		manager.mu.Lock()
		for _, w := range manager.sessions["demo"].Windows {
			if w != nil && w.ID == windowID {
				w.Panes = nil
				break
			}
		}
		manager.mu.Unlock()

		err = manager.ApplyLayoutPresetByWindowID("demo", windowID, PresetEvenHorizontal)
		if err == nil {
			t.Fatal("expected error for window with no panes, got nil")
		}
		if !strings.Contains(err.Error(), "no panes") {
			t.Fatalf("error = %q, want containing 'no panes'", err.Error())
		}
	})

	t.Run("window with all nil pane entries returns error", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		manager.mu.RLock()
		windowID := pane.Window.ID
		manager.mu.RUnlock()

		// Replace panes with nil entries.
		manager.mu.Lock()
		for _, w := range manager.sessions["demo"].Windows {
			if w != nil && w.ID == windowID {
				w.Panes = []*TmuxPane{nil, nil}
				break
			}
		}
		manager.mu.Unlock()

		err = manager.ApplyLayoutPresetByWindowID("demo", windowID, PresetEvenHorizontal)
		if err == nil {
			t.Fatal("expected error for window with all nil pane entries, got nil")
		}
		if !strings.Contains(err.Error(), "no valid panes") {
			t.Fatalf("error = %q, want containing 'no valid panes'", err.Error())
		}
	})

	t.Run("multiple presets applied sequentially", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if _, err := manager.SplitPane(pane0.ID, SplitHorizontal); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		manager.mu.RLock()
		windowID := pane0.Window.ID
		manager.mu.RUnlock()

		presets := []LayoutPreset{
			PresetEvenHorizontal,
			PresetEvenVertical,
			PresetMainVertical,
			PresetMainHorizontal,
			PresetTiled,
		}
		for _, preset := range presets {
			t.Run(string(preset), func(t *testing.T) {
				err := manager.ApplyLayoutPresetByWindowID("demo", windowID, preset)
				if err != nil {
					t.Fatalf("ApplyLayoutPresetByWindowID(%s) error = %v", preset, err)
				}
				snapshots := manager.Snapshot()
				if snapshots[0].Windows[0].Layout == nil {
					t.Fatalf("layout is nil after applying preset %s", preset)
				}
			})
		}
	})

	t.Run("topology generation advances after preset application", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if _, err := manager.SplitPane(pane0.ID, SplitHorizontal); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		manager.mu.RLock()
		windowID := pane0.Window.ID
		manager.mu.RUnlock()

		beforeGen := manager.TopologyGeneration()

		err = manager.ApplyLayoutPresetByWindowID("demo", windowID, PresetEvenVertical)
		if err != nil {
			t.Fatalf("ApplyLayoutPresetByWindowID() error = %v", err)
		}

		afterGen := manager.TopologyGeneration()
		if afterGen <= beforeGen {
			t.Fatalf("topology generation did not advance: before=%d after=%d", beforeGen, afterGen)
		}
	})

	t.Run("nil window entry in session windows is skipped", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if _, err := manager.SplitPane(pane0.ID, SplitHorizontal); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		manager.mu.RLock()
		windowID := pane0.Window.ID
		manager.mu.RUnlock()

		// Inject a nil window entry before the real window.
		manager.mu.Lock()
		session := manager.sessions["demo"]
		session.Windows = append([]*TmuxWindow{nil}, session.Windows...)
		manager.mu.Unlock()

		err = manager.ApplyLayoutPresetByWindowID("demo", windowID, PresetEvenHorizontal)
		if err != nil {
			t.Fatalf("ApplyLayoutPresetByWindowID() should skip nil window entries, got error = %v", err)
		}
	})
}
