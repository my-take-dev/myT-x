package tmux

import (
	"strings"
	"testing"
)

// I-06: Table-driven tests for ResolveDirectionalPane covering:
// - 2x2 layout up/down/left/right movement
// - Single pane returns self
// - Invalid direction returns error
// - 3+ pane mid-pane selection
// - Edge clamping behaviour
// - Negative callerPaneID uses default pane
// - Missing caller pane returns error

func TestResolveDirectionalPane(t *testing.T) {
	t.Run("single pane returns self for all directions", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		directions := []struct {
			name string
			dir  DirectionalPaneDirection
		}{
			{"DirNone", DirNone},
			{"DirPrev", DirPrev},
			{"DirNext", DirNext},
		}

		for _, dd := range directions {
			t.Run(dd.name, func(t *testing.T) {
				resolved, resolveErr := manager.ResolveDirectionalPane(pane.ID, dd.dir)
				if resolveErr != nil {
					t.Fatalf("ResolveDirectionalPane(%s) error = %v", dd.name, resolveErr)
				}
				if resolved.ID != pane.ID {
					t.Fatalf("ResolveDirectionalPane(%s) pane ID = %d, want %d", dd.name, resolved.ID, pane.ID)
				}
			})
		}
	})

	t.Run("two panes DirNext moves to second pane", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		resolved, resolveErr := manager.ResolveDirectionalPane(pane0.ID, DirNext)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNext) error = %v", resolveErr)
		}
		if resolved.ID != pane1.ID {
			t.Fatalf("pane ID = %d, want %d (next pane)", resolved.ID, pane1.ID)
		}
	})

	t.Run("two panes DirPrev moves to first pane", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		resolved, resolveErr := manager.ResolveDirectionalPane(pane1.ID, DirPrev)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirPrev) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (prev pane)", resolved.ID, pane0.ID)
		}
	})

	t.Run("edge clamping: DirPrev on first pane returns self", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if _, err := manager.SplitPane(pane0.ID, SplitHorizontal); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		resolved, resolveErr := manager.ResolveDirectionalPane(pane0.ID, DirPrev)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirPrev) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (clamped to self)", resolved.ID, pane0.ID)
		}
	})

	t.Run("edge clamping: DirNext on last pane returns self", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		resolved, resolveErr := manager.ResolveDirectionalPane(pane1.ID, DirNext)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNext) error = %v", resolveErr)
		}
		if resolved.ID != pane1.ID {
			t.Fatalf("pane ID = %d, want %d (clamped to self)", resolved.ID, pane1.ID)
		}
	})

	t.Run("three panes mid-pane DirPrev", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane(1) error = %v", err)
		}
		_, err = manager.SplitPane(pane1.ID, SplitVertical)
		if err != nil {
			t.Fatalf("SplitPane(2) error = %v", err)
		}

		// pane1 is the middle pane. DirPrev should move to pane0.
		resolved, resolveErr := manager.ResolveDirectionalPane(pane1.ID, DirPrev)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirPrev) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (previous from mid)", resolved.ID, pane0.ID)
		}
	})

	t.Run("three panes mid-pane DirNext", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane(1) error = %v", err)
		}
		pane2, err := manager.SplitPane(pane1.ID, SplitVertical)
		if err != nil {
			t.Fatalf("SplitPane(2) error = %v", err)
		}

		// pane1 is the middle pane. DirNext should move to pane2.
		resolved, resolveErr := manager.ResolveDirectionalPane(pane1.ID, DirNext)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNext) error = %v", resolveErr)
		}
		if resolved.ID != pane2.ID {
			t.Fatalf("pane ID = %d, want %d (next from mid)", resolved.ID, pane2.ID)
		}
	})

	t.Run("DirNone returns current pane unchanged", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if _, err := manager.SplitPane(pane0.ID, SplitHorizontal); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		resolved, resolveErr := manager.ResolveDirectionalPane(pane0.ID, DirNone)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNone) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (DirNone)", resolved.ID, pane0.ID)
		}
	})

	t.Run("caller pane not found returns error", func(t *testing.T) {
		manager := NewSessionManager()
		if _, _, err := manager.CreateSession("demo", "main", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		_, resolveErr := manager.ResolveDirectionalPane(9999, DirNext)
		if resolveErr == nil {
			t.Fatal("expected error for non-existent pane, got nil")
		}
		if !strings.Contains(resolveErr.Error(), "caller pane not found") {
			t.Fatalf("error = %q, want containing 'caller pane not found'", resolveErr.Error())
		}
	})

	t.Run("negative callerPaneID uses default pane", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		// DirNone with negative callerPaneID should resolve default pane.
		resolved, resolveErr := manager.ResolveDirectionalPane(-1, DirNone)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(-1, DirNone) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (default pane)", resolved.ID, pane0.ID)
		}
	})

	t.Run("negative callerPaneID no sessions returns error", func(t *testing.T) {
		manager := NewSessionManager()

		_, resolveErr := manager.ResolveDirectionalPane(-1, DirNext)
		if resolveErr == nil {
			t.Fatal("expected error for empty session manager, got nil")
		}
	})
}

// TestResolveDirectionalPane_FourPaneLayout tests directional navigation in a
// 2x2 layout (4 panes). Panes are arranged as [p0, p1, p2, p3] in the
// window's Panes slice, so DirPrev/DirNext navigate sequentially.
func TestResolveDirectionalPane_FourPaneLayout(t *testing.T) {
	manager := NewSessionManager()
	_, p0, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	p1, err := manager.SplitPane(p0.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane(1) error = %v", err)
	}
	p2, err := manager.SplitPane(p1.ID, SplitVertical)
	if err != nil {
		t.Fatalf("SplitPane(2) error = %v", err)
	}
	p3, err := manager.SplitPane(p0.ID, SplitVertical)
	if err != nil {
		t.Fatalf("SplitPane(3) error = %v", err)
	}

	// Verify we have 4 panes in the window.
	manager.mu.RLock()
	windowPanes := p0.Window.Panes
	manager.mu.RUnlock()
	if len(windowPanes) != 4 {
		t.Fatalf("pane count = %d, want 4", len(windowPanes))
	}

	tests := []struct {
		name       string
		callerID   int
		direction  DirectionalPaneDirection
		wantPaneID int
	}{
		// DirNext from each position
		{"p0 DirNext -> p1", p0.ID, DirNext, p1.ID},
		{"p1 DirNext -> p2", p1.ID, DirNext, p2.ID},
		{"p2 DirNext -> p3", p2.ID, DirNext, p3.ID},
		{"p3 DirNext -> p3 (clamped)", p3.ID, DirNext, p3.ID},

		// DirPrev from each position
		{"p0 DirPrev -> p0 (clamped)", p0.ID, DirPrev, p0.ID},
		{"p1 DirPrev -> p0", p1.ID, DirPrev, p0.ID},
		{"p2 DirPrev -> p1", p2.ID, DirPrev, p1.ID},
		{"p3 DirPrev -> p2", p3.ID, DirPrev, p2.ID},

		// DirNone returns self
		{"p0 DirNone -> p0", p0.ID, DirNone, p0.ID},
		{"p2 DirNone -> p2", p2.ID, DirNone, p2.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, resolveErr := manager.ResolveDirectionalPane(tt.callerID, tt.direction)
			if resolveErr != nil {
				t.Fatalf("ResolveDirectionalPane() error = %v", resolveErr)
			}
			if resolved.ID != tt.wantPaneID {
				t.Fatalf("pane ID = %d, want %d", resolved.ID, tt.wantPaneID)
			}
		})
	}
}

// I-15: TestResolveDirectionalPane_NilPaneEntriesSkipped verifies that nil
// entries in the Panes slice are correctly skipped during directional navigation.
// This covers the defensive nil-skip path in the DirNext/DirPrev scan loop.
func TestResolveDirectionalPane_NilPaneEntriesSkipped(t *testing.T) {
	t.Run("DirNext skips nil pane and finds next valid pane", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane(1) error = %v", err)
		}
		pane2, err := manager.SplitPane(pane1.ID, SplitVertical)
		if err != nil {
			t.Fatalf("SplitPane(2) error = %v", err)
		}

		// Inject nil entry at position of pane1 (index 1).
		// After this, Panes = [pane0, nil, pane2].
		manager.mu.Lock()
		window := pane0.Window
		window.Panes[1] = nil
		manager.mu.Unlock()

		// DirNext from pane0: idx 1 is nil, should skip to pane2 at idx 2.
		resolved, resolveErr := manager.ResolveDirectionalPane(pane0.ID, DirNext)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNext) error = %v", resolveErr)
		}
		if resolved.ID != pane2.ID {
			t.Fatalf("pane ID = %d, want %d (should skip nil pane1)", resolved.ID, pane2.ID)
		}
	})

	t.Run("DirPrev skips nil pane and finds previous valid pane", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane(1) error = %v", err)
		}
		pane2, err := manager.SplitPane(pane1.ID, SplitVertical)
		if err != nil {
			t.Fatalf("SplitPane(2) error = %v", err)
		}

		// Inject nil entry at position of pane1 (index 1).
		// After this, Panes = [pane0, nil, pane2].
		manager.mu.Lock()
		window := pane0.Window
		window.Panes[1] = nil
		manager.mu.Unlock()

		// DirPrev from pane2: idx 1 is nil, should skip to pane0 at idx 0.
		resolved, resolveErr := manager.ResolveDirectionalPane(pane2.ID, DirPrev)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirPrev) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (should skip nil pane1)", resolved.ID, pane0.ID)
		}
	})

	t.Run("all surrounding panes nil falls back to current pane", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane(1) error = %v", err)
		}
		pane2, err := manager.SplitPane(pane1.ID, SplitVertical)
		if err != nil {
			t.Fatalf("SplitPane(2) error = %v", err)
		}
		_ = pane2

		// Make pane1 (idx 1) and pane2 (idx 2) nil, leaving only pane0.
		// Panes = [pane0, nil, nil]
		manager.mu.Lock()
		window := pane0.Window
		window.Panes[1] = nil
		window.Panes[2] = nil
		manager.mu.Unlock()

		// DirNext from pane0: idx 1 and 2 are nil, should return pane0 (edge clamp + nil skip).
		resolved, resolveErr := manager.ResolveDirectionalPane(pane0.ID, DirNext)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNext) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d (all next panes are nil, should fall back to current)", resolved.ID, pane0.ID)
		}
	})

	t.Run("DirNone with nil panes returns current pane unchanged", func(t *testing.T) {
		manager := NewSessionManager()
		_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}

		// Nil-ify pane1. DirNone should still return current pane (pane0).
		manager.mu.Lock()
		pane0.Window.Panes[1] = nil
		manager.mu.Unlock()
		_ = pane1

		resolved, resolveErr := manager.ResolveDirectionalPane(pane0.ID, DirNone)
		if resolveErr != nil {
			t.Fatalf("ResolveDirectionalPane(DirNone) error = %v", resolveErr)
		}
		if resolved.ID != pane0.ID {
			t.Fatalf("pane ID = %d, want %d", resolved.ID, pane0.ID)
		}
	})
}

// C-3: TestResolveDirectionalPane_MultipleWindowScopeIsolation verifies that
// DirNext/DirPrev navigation does NOT cross window boundaries. Even in the
// 1-window model, this invariant must hold: pane navigation is scoped to the
// containing window's Panes slice, not the global pane map.
//
// Multi-window state is constructed by directly manipulating internal fields
// because AddWindow was removed in the 1-window model refactor.
func TestResolveDirectionalPane_MultipleWindowScopeIsolation(t *testing.T) {
	manager := NewSessionManager()
	_, pane0, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Split to create a second pane in window 0.
	pane1, err := manager.SplitPane(pane0.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	// Inject a second window with its own pane into the same session.
	_, isolatedPane := injectTestWindow(t, manager, "demo", "isolated")

	// DirNext from the last pane in window 0 should NOT cross into window 2.
	resolved, resolveErr := manager.ResolveDirectionalPane(pane1.ID, DirNext)
	if resolveErr != nil {
		t.Fatalf("ResolveDirectionalPane(DirNext) error = %v", resolveErr)
	}
	if resolved.ID == isolatedPane.ID {
		t.Fatalf("DirNext crossed window boundary: resolved pane %d is in window 2, expected to stay in window 0", resolved.ID)
	}
	if resolved.ID != pane1.ID {
		t.Fatalf("DirNext from last pane: resolved ID = %d, want %d (clamped to self)", resolved.ID, pane1.ID)
	}

	// DirPrev from the first pane in window 0 should NOT cross into window 2.
	resolved, resolveErr = manager.ResolveDirectionalPane(pane0.ID, DirPrev)
	if resolveErr != nil {
		t.Fatalf("ResolveDirectionalPane(DirPrev) error = %v", resolveErr)
	}
	if resolved.ID == isolatedPane.ID {
		t.Fatalf("DirPrev crossed window boundary: resolved pane %d is in window 2", resolved.ID)
	}
	if resolved.ID != pane0.ID {
		t.Fatalf("DirPrev from first pane: resolved ID = %d, want %d (clamped to self)", resolved.ID, pane0.ID)
	}

	// DirNext from the isolated pane in window 2 should stay in window 2.
	resolved, resolveErr = manager.ResolveDirectionalPane(isolatedPane.ID, DirNext)
	if resolveErr != nil {
		t.Fatalf("ResolveDirectionalPane(DirNext) from isolated pane error = %v", resolveErr)
	}
	if resolved.ID != isolatedPane.ID {
		t.Fatalf("DirNext from sole pane in window 2: resolved ID = %d, want %d (clamped to self)", resolved.ID, isolatedPane.ID)
	}

	// DirPrev from the isolated pane in window 2 should stay in window 2.
	resolved, resolveErr = manager.ResolveDirectionalPane(isolatedPane.ID, DirPrev)
	if resolveErr != nil {
		t.Fatalf("ResolveDirectionalPane(DirPrev) from isolated pane error = %v", resolveErr)
	}
	if resolved.ID != isolatedPane.ID {
		t.Fatalf("DirPrev from sole pane in window 2: resolved ID = %d, want %d (clamped to self)", resolved.ID, isolatedPane.ID)
	}
}

// TestResolveDirectionalPane_PaneWithNilWindow tests the defensive error path
// when a pane's Window reference is unexpectedly nil.
func TestResolveDirectionalPane_PaneWithNilWindow(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Simulate a corrupted state where the pane's Window is nil.
	manager.mu.Lock()
	pane.Window = nil
	manager.mu.Unlock()

	_, resolveErr := manager.ResolveDirectionalPane(pane.ID, DirNext)
	if resolveErr == nil {
		t.Fatal("expected error for nil window, got nil")
	}
	if !strings.Contains(resolveErr.Error(), "no window") {
		t.Fatalf("error = %q, want containing 'no window'", resolveErr.Error())
	}
}
