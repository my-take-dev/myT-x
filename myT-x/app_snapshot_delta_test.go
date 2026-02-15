package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

func testSnapshot(name string, id int, idle bool) tmux.SessionSnapshot {
	return tmux.SessionSnapshot{
		ID:        id,
		Name:      name,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsIdle:    idle,
		Windows:   []tmux.WindowSnapshot{},
	}
}

func testSnapshotWithPane(name string, paneTitle string) tmux.SessionSnapshot {
	return tmux.SessionSnapshot{
		ID:        1,
		Name:      name,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		IsIdle:    false,
		Windows: []tmux.WindowSnapshot{
			{
				ID:       0,
				Name:     "0",
				ActivePN: 0,
				Panes: []tmux.PaneSnapshot{
					{
						ID:     "%0",
						Index:  0,
						Title:  paneTitle,
						Active: true,
						Width:  120,
						Height: 40,
					},
				},
			},
		},
	}
}

func TestSnapshotDeltaInitialUsesFullSnapshot(t *testing.T) {
	app := NewApp()

	delta, changed, initial := app.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("alpha", 1, false),
	})
	if changed {
		t.Fatal("initial delta should not be marked as changed")
	}
	if !initial {
		t.Fatal("initial snapshot should request full emit")
	}
	if len(delta.Upserts) != 0 || len(delta.Removed) != 0 {
		t.Fatalf("initial delta should be empty: %#v", delta)
	}
}

func TestSnapshotDeltaDetectsUpsertAndRemoval(t *testing.T) {
	app := NewApp()
	_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("alpha", 1, false),
		testSnapshot("beta", 2, false),
	})

	delta, changed, initial := app.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("alpha", 1, true),
		testSnapshot("gamma", 3, false),
	})
	if initial {
		t.Fatal("second snapshot should not be marked as initial")
	}
	if !changed {
		t.Fatal("delta should report changes")
	}
	if len(delta.Upserts) != 2 {
		t.Fatalf("upserts length = %d, want 2", len(delta.Upserts))
	}
	if len(delta.Removed) != 1 || delta.Removed[0] != "beta" {
		t.Fatalf("removed = %#v, want [beta]", delta.Removed)
	}
}

func TestSnapshotDeltaDetectsNoChange(t *testing.T) {
	app := NewApp()
	initial := []tmux.SessionSnapshot{testSnapshotWithPane("alpha", "editor")}
	_, _, _ = app.snapshotDelta(initial)

	delta, changed, initialFlag := app.snapshotDelta([]tmux.SessionSnapshot{testSnapshotWithPane("alpha", "editor")})
	if initialFlag {
		t.Fatal("second snapshot should not be marked as initial")
	}
	if changed {
		t.Fatalf("delta should not report changes: %#v", delta)
	}
	if len(delta.Upserts) != 0 || len(delta.Removed) != 0 {
		t.Fatalf("delta should be empty: %#v", delta)
	}
}

func TestSnapshotDeltaDetectsNestedPaneChange(t *testing.T) {
	app := NewApp()
	_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{testSnapshotWithPane("alpha", "editor")})

	delta, changed, initial := app.snapshotDelta([]tmux.SessionSnapshot{testSnapshotWithPane("alpha", "build")})
	if initial {
		t.Fatal("second snapshot should not be marked as initial")
	}
	if !changed {
		t.Fatal("delta should report changes")
	}
	if len(delta.Upserts) != 1 {
		t.Fatalf("upserts length = %d, want 1", len(delta.Upserts))
	}
}

// TestSnapshotFieldCounts guards against field drift in hand-written equality
// functions. If a field is added to a snapshot struct, the corresponding
// equality function must be updated, and this test will fail as a reminder.
func TestSnapshotFieldCounts(t *testing.T) {
	tests := []struct {
		name     string
		typ      any
		expected int
	}{
		{"TmuxSession", tmux.TmuxSession{}, 10},
		{"SessionSnapshot", tmux.SessionSnapshot{}, 8},
		{"SessionWorktreeInfo", tmux.SessionWorktreeInfo{}, 5},
		{"PaneContextSnapshot", tmux.PaneContextSnapshot{}, 5},
		{"WindowSnapshot", tmux.WindowSnapshot{}, 5},
		{"PaneSnapshot", tmux.PaneSnapshot{}, 6},
		{"LayoutNode", tmux.LayoutNode{}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := reflect.TypeOf(tt.typ).NumField()
			if n != tt.expected {
				t.Fatalf("%s has %d fields (expected %d); update the corresponding *Equal and estimate* functions", tt.name, n, tt.expected)
			}
		})
	}
}

func TestSnapshotDeltaDetectsLayoutChange(t *testing.T) {
	makeSnapshot := func(ratio float64) tmux.SessionSnapshot {
		return tmux.SessionSnapshot{
			ID:        1,
			Name:      "alpha",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Windows: []tmux.WindowSnapshot{
				{
					ID:       0,
					Name:     "0",
					ActivePN: 0,
					Layout: &tmux.LayoutNode{
						Type:      tmux.LayoutSplit,
						Direction: tmux.SplitHorizontal,
						Ratio:     ratio,
						Children: [2]*tmux.LayoutNode{
							{Type: tmux.LayoutLeaf, PaneID: 0},
							{Type: tmux.LayoutLeaf, PaneID: 1},
						},
					},
					Panes: []tmux.PaneSnapshot{
						{ID: "%0", Index: 0, Active: true, Width: 60, Height: 40},
						{ID: "%1", Index: 1, Active: false, Width: 60, Height: 40},
					},
				},
			},
		}
	}

	app := NewApp()
	_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{makeSnapshot(0.5)})

	// Same layout - no delta
	_, changed, _ := app.snapshotDelta([]tmux.SessionSnapshot{makeSnapshot(0.5)})
	if changed {
		t.Fatal("identical layout should not produce delta")
	}

	// Different ratio - should produce delta
	delta, changed, _ := app.snapshotDelta([]tmux.SessionSnapshot{makeSnapshot(0.7)})
	if !changed {
		t.Fatal("layout ratio change should produce delta")
	}
	if len(delta.Upserts) != 1 {
		t.Fatalf("upserts length = %d, want 1", len(delta.Upserts))
	}
}

func TestLayoutSnapshotEqualDetectsStructuralChanges(t *testing.T) {
	makeLayout := func(direction tmux.SplitDirection, leftPaneID int, rightPaneID int) *tmux.LayoutNode {
		return &tmux.LayoutNode{
			Type:      tmux.LayoutSplit,
			Direction: direction,
			Ratio:     0.5,
			Children: [2]*tmux.LayoutNode{
				{Type: tmux.LayoutLeaf, PaneID: leftPaneID},
				{Type: tmux.LayoutLeaf, PaneID: rightPaneID},
			},
		}
	}

	base := makeLayout(tmux.SplitHorizontal, 0, 1)
	if !layoutSnapshotEqual(base, makeLayout(tmux.SplitHorizontal, 0, 1)) {
		t.Fatal("identical layouts should be equal")
	}

	tests := []struct {
		name  string
		other *tmux.LayoutNode
	}{
		{"direction differs", makeLayout(tmux.SplitVertical, 0, 1)},
		{"left pane differs", makeLayout(tmux.SplitHorizontal, 9, 1)},
		{"right pane differs", makeLayout(tmux.SplitHorizontal, 0, 9)},
		{
			name: "child structure differs",
			other: &tmux.LayoutNode{
				Type:      tmux.LayoutSplit,
				Direction: tmux.SplitHorizontal,
				Ratio:     0.5,
				Children: [2]*tmux.LayoutNode{
					{
						Type:      tmux.LayoutSplit,
						Direction: tmux.SplitVertical,
						Ratio:     0.5,
						Children: [2]*tmux.LayoutNode{
							{Type: tmux.LayoutLeaf, PaneID: 0},
							{Type: tmux.LayoutLeaf, PaneID: 1},
						},
					},
					{Type: tmux.LayoutLeaf, PaneID: 2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if layoutSnapshotEqual(base, tt.other) {
				t.Fatalf("layoutSnapshotEqual should detect %s", tt.name)
			}
		})
	}
}

func TestSnapshotDeltaDetectsRootPathChange(t *testing.T) {
	app := NewApp()
	snap := testSnapshot("alpha", 1, false)
	snap.RootPath = "/old/path"
	_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{snap})

	snap2 := testSnapshot("alpha", 1, false)
	snap2.RootPath = "/new/path"
	delta, changed, initial := app.snapshotDelta([]tmux.SessionSnapshot{snap2})
	if initial {
		t.Fatal("second snapshot should not be marked as initial")
	}
	if !changed {
		t.Fatal("RootPath change should produce delta")
	}
	if len(delta.Upserts) != 1 {
		t.Fatalf("upserts length = %d, want 1", len(delta.Upserts))
	}
	if delta.Upserts[0].RootPath != "/new/path" {
		t.Fatalf("upserted RootPath = %q, want /new/path", delta.Upserts[0].RootPath)
	}
}

func TestSnapshotDeltaDetectsWorktreeChange(t *testing.T) {
	base := tmux.SessionWorktreeInfo{
		Path:       "/repo/.wt/feature-a",
		RepoPath:   "/repo",
		BranchName: "feature-a",
		BaseBranch: "main",
		IsDetached: false,
	}

	tests := []struct {
		name   string
		mutate func(*tmux.SessionWorktreeInfo)
	}{
		{
			name: "path changes",
			mutate: func(info *tmux.SessionWorktreeInfo) {
				info.Path = "/repo/.wt/feature-b"
			},
		},
		{
			name: "repo path changes",
			mutate: func(info *tmux.SessionWorktreeInfo) {
				info.RepoPath = "/repo-2"
			},
		},
		{
			name: "branch changes",
			mutate: func(info *tmux.SessionWorktreeInfo) {
				info.BranchName = "feature-b"
			},
		},
		{
			name: "base branch changes",
			mutate: func(info *tmux.SessionWorktreeInfo) {
				info.BaseBranch = "release"
			},
		},
		{
			name: "detached flag changes",
			mutate: func(info *tmux.SessionWorktreeInfo) {
				info.IsDetached = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()

			snap := testSnapshot("alpha", 1, false)
			initialWorktree := base
			snap.Worktree = &initialWorktree
			_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{snap})

			changed := testSnapshot("alpha", 1, false)
			nextWorktree := base
			tt.mutate(&nextWorktree)
			changed.Worktree = &nextWorktree

			delta, hasChanges, initial := app.snapshotDelta([]tmux.SessionSnapshot{changed})
			if initial {
				t.Fatal("second snapshot should not be marked as initial")
			}
			if !hasChanges {
				t.Fatal("worktree change should produce delta")
			}
			if len(delta.Upserts) != 1 {
				t.Fatalf("upserts length = %d, want 1", len(delta.Upserts))
			}
			if delta.Upserts[0].Worktree == nil {
				t.Fatal("upserted worktree is nil, want non-nil")
			}
		})
	}
}

func TestSnapshotDeltaDetectsWorktreeNilTransitions(t *testing.T) {
	t.Run("nil to non-nil", func(t *testing.T) {
		app := NewApp()
		initial := testSnapshot("alpha", 1, false)
		_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{initial})

		changed := testSnapshot("alpha", 1, false)
		changed.Worktree = &tmux.SessionWorktreeInfo{
			Path:       "/repo/.wt/feature-a",
			RepoPath:   "/repo",
			BranchName: "feature-a",
			BaseBranch: "main",
			IsDetached: false,
		}

		delta, hasChanges, first := app.snapshotDelta([]tmux.SessionSnapshot{changed})
		if first {
			t.Fatal("second snapshot should not be marked as initial")
		}
		if !hasChanges {
			t.Fatal("nil to non-nil worktree should produce delta")
		}
		if len(delta.Upserts) != 1 {
			t.Fatalf("upserts length = %d, want 1", len(delta.Upserts))
		}
	})

	t.Run("non-nil to nil", func(t *testing.T) {
		app := NewApp()
		initial := testSnapshot("alpha", 1, false)
		initial.Worktree = &tmux.SessionWorktreeInfo{
			Path:       "/repo/.wt/feature-a",
			RepoPath:   "/repo",
			BranchName: "feature-a",
			BaseBranch: "main",
			IsDetached: false,
		}
		_, _, _ = app.snapshotDelta([]tmux.SessionSnapshot{initial})

		changed := testSnapshot("alpha", 1, false)
		changed.Worktree = nil

		delta, hasChanges, first := app.snapshotDelta([]tmux.SessionSnapshot{changed})
		if first {
			t.Fatal("second snapshot should not be marked as initial")
		}
		if !hasChanges {
			t.Fatal("non-nil to nil worktree should produce delta")
		}
		if len(delta.Upserts) != 1 {
			t.Fatalf("upserts length = %d, want 1", len(delta.Upserts))
		}
		if delta.Upserts[0].Worktree != nil {
			t.Fatalf("upserted worktree = %#v, want nil", delta.Upserts[0].Worktree)
		}
	})
}

func TestSnapshotDeltaSequentialOperations(t *testing.T) {
	app := NewApp()

	// Step 1: Prime with sessions A and B.
	initial := []tmux.SessionSnapshot{
		testSnapshot("alpha", 1, false),
		testSnapshot("beta", 2, false),
	}
	_, _, isInitial := app.snapshotDelta(initial)
	if !isInitial {
		t.Fatal("first call should be initial")
	}

	// Step 2: Add session C.
	withC := []tmux.SessionSnapshot{
		testSnapshot("alpha", 1, false),
		testSnapshot("beta", 2, false),
		testSnapshot("gamma", 3, false),
	}
	delta, changed, _ := app.snapshotDelta(withC)
	if !changed {
		t.Fatal("adding gamma should produce delta")
	}
	if len(delta.Upserts) != 1 || delta.Upserts[0].Name != "gamma" {
		t.Fatalf("expected gamma upsert, got %#v", delta.Upserts)
	}
	if len(delta.Removed) != 0 {
		t.Fatalf("expected no removals, got %#v", delta.Removed)
	}

	// Step 3: Remove session B.
	withoutB := []tmux.SessionSnapshot{
		testSnapshot("alpha", 1, false),
		testSnapshot("gamma", 3, false),
	}
	delta, changed, _ = app.snapshotDelta(withoutB)
	if !changed {
		t.Fatal("removing beta should produce delta")
	}
	if len(delta.Removed) != 1 || delta.Removed[0] != "beta" {
		t.Fatalf("expected beta removal, got %#v", delta.Removed)
	}
	if len(delta.Upserts) != 0 {
		t.Fatalf("expected no upserts, got %#v", delta.Upserts)
	}

	// Step 4: Add session B back.
	withBAgain := []tmux.SessionSnapshot{
		testSnapshot("alpha", 1, false),
		testSnapshot("beta", 2, true), // changed IsIdle
		testSnapshot("gamma", 3, false),
	}
	delta, changed, _ = app.snapshotDelta(withBAgain)
	if !changed {
		t.Fatal("adding beta back should produce delta")
	}
	if len(delta.Upserts) != 1 || delta.Upserts[0].Name != "beta" {
		t.Fatalf("expected beta upsert, got %#v", delta.Upserts)
	}
	if len(delta.Removed) != 0 {
		t.Fatalf("expected no removals, got %#v", delta.Removed)
	}

	// Step 5: No change — cache should be consistent.
	delta, changed, _ = app.snapshotDelta(withBAgain)
	if changed {
		t.Fatalf("same snapshot should not produce delta: %#v", delta)
	}
}

func BenchmarkSnapshotDelta(b *testing.B) {
	for _, count := range []int{10, 50} {
		b.Run(fmt.Sprintf("sessions=%d/no_change", count), func(b *testing.B) {
			snapshots := make([]tmux.SessionSnapshot, count)
			for i := range snapshots {
				snapshots[i] = testSnapshot(fmt.Sprintf("session-%d", i), i, false)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				app := NewApp()
				app.snapshotDelta(snapshots) // prime
				app.snapshotDelta(snapshots) // steady state (no changes)
			}
		})

		b.Run(fmt.Sprintf("sessions=%d/with_upsert", count), func(b *testing.B) {
			base := make([]tmux.SessionSnapshot, count)
			for i := range base {
				base[i] = testSnapshot(fmt.Sprintf("session-%d", i), i, false)
			}
			changed := make([]tmux.SessionSnapshot, count)
			copy(changed, base)
			// Toggle IsIdle on last session to trigger upsert.
			changed[count-1] = testSnapshot(fmt.Sprintf("session-%d", count-1), count-1, true)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				app := NewApp()
				app.snapshotDelta(base)    // prime
				app.snapshotDelta(changed) // with upsert
			}
		})
	}
}
