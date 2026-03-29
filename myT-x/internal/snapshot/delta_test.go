package snapshot

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testSnapshot(name string, id int, idle bool) tmux.SessionSnapshot {
	return tmux.SessionSnapshot{
		Name:   name,
		ID:     id,
		IsIdle: idle,
	}
}

func testSnapshotWithPane(name string, paneTitle string) tmux.SessionSnapshot {
	return tmux.SessionSnapshot{
		Name: name,
		ID:   1,
		Windows: []tmux.WindowSnapshot{
			{
				ID:   1,
				Name: "win1",
				Panes: []tmux.PaneSnapshot{
					{ID: "%0", Index: 0, Title: paneTitle, Active: true, Width: 80, Height: 24},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// snapshotDelta basics
// ---------------------------------------------------------------------------

func TestSnapshotDeltaInitialUsesFullSnapshot(t *testing.T) {
	svc := newTestService(t)
	snapshots := []tmux.SessionSnapshot{testSnapshot("s1", 1, false)}

	delta, changed, initial := svc.snapshotDelta(snapshots)
	if !initial {
		t.Fatal("first call should return initial=true")
	}
	if changed {
		t.Error("initial call should return changed=false")
	}
	_ = delta
}

func TestSnapshotDeltaDetectsUpsertAndRemoval(t *testing.T) {
	svc := newTestService(t)

	// Seed.
	svc.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
		testSnapshot("s2", 2, false),
	})

	// Remove s1, add s3.
	delta, changed, initial := svc.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("s2", 2, false),
		testSnapshot("s3", 3, false),
	})
	if initial {
		t.Error("second call should not be initial")
	}
	if !changed {
		t.Fatal("should detect changes")
	}
	if len(delta.Upserts) != 1 || delta.Upserts[0].Name != "s3" {
		t.Errorf("upserts = %v, want [s3]", delta.Upserts)
	}
	if len(delta.Removed) != 1 || delta.Removed[0] != "s1" {
		t.Errorf("removed = %v, want [s1]", delta.Removed)
	}
}

func TestSnapshotDeltaDetectsNoChange(t *testing.T) {
	svc := newTestService(t)

	snap := []tmux.SessionSnapshot{testSnapshot("s1", 1, false)}
	svc.snapshotDelta(snap) // seed

	_, changed, initial := svc.snapshotDelta(snap)
	if initial {
		t.Error("should not be initial")
	}
	if changed {
		t.Error("identical snapshot should report no change")
	}
}

func TestSnapshotDeltaDetectsNestedPaneChange(t *testing.T) {
	svc := newTestService(t)

	svc.snapshotDelta([]tmux.SessionSnapshot{testSnapshotWithPane("s1", "old-title")})

	delta, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{testSnapshotWithPane("s1", "new-title")})
	if !changed {
		t.Fatal("pane title change should be detected")
	}
	if len(delta.Upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(delta.Upserts))
	}
}

func TestSnapshotDeltaDetectsActiveWindowIDChange(t *testing.T) {
	svc := newTestService(t)

	base := testSnapshot("s1", 1, false)
	base.ActiveWindowID = 10
	svc.snapshotDelta([]tmux.SessionSnapshot{base})

	modified := testSnapshot("s1", 1, false)
	modified.ActiveWindowID = 20
	delta, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{modified})
	if !changed {
		t.Fatal("ActiveWindowID change should be detected")
	}
	if len(delta.Upserts) != 1 {
		t.Errorf("expected 1 upsert, got %d", len(delta.Upserts))
	}
}

// ---------------------------------------------------------------------------
// Field count guards
// ---------------------------------------------------------------------------

func TestSnapshotFieldCounts(t *testing.T) {
	tests := []struct {
		name       string
		typ        reflect.Type
		wantFields int
	}{
		{"TmuxSession", reflect.TypeFor[tmux.TmuxSession](), 14},
		{"SessionSnapshot", reflect.TypeFor[tmux.SessionSnapshot](), 9},
		{"SessionWorktreeInfo", reflect.TypeFor[tmux.SessionWorktreeInfo](), 5},
		{"PaneContextSnapshot", reflect.TypeFor[tmux.PaneContextSnapshot](), 9},
		{"WindowSnapshot", reflect.TypeFor[tmux.WindowSnapshot](), 5},
		{"PaneSnapshot", reflect.TypeFor[tmux.PaneSnapshot](), 6},
		{"LayoutNode", reflect.TypeFor[tmux.LayoutNode](), 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.NumField()
			if got != tt.wantFields {
				t.Errorf("%s has %d fields, want %d; update equality helpers and this test", tt.name, got, tt.wantFields)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Layout equality
// ---------------------------------------------------------------------------

func TestSnapshotDeltaDetectsLayoutChange(t *testing.T) {
	svc := newTestService(t)

	makeSnap := func(layout *tmux.LayoutNode) tmux.SessionSnapshot {
		return tmux.SessionSnapshot{
			Name: "s1",
			ID:   1,
			Windows: []tmux.WindowSnapshot{
				{
					ID:     1,
					Name:   "win1",
					Layout: layout,
					Panes:  []tmux.PaneSnapshot{{ID: "%0", Active: true, Width: 80, Height: 24}},
				},
			},
		}
	}

	svc.snapshotDelta([]tmux.SessionSnapshot{makeSnap(&tmux.LayoutNode{
		Type: tmux.LayoutLeaf, PaneID: 0,
	})})

	_, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{makeSnap(&tmux.LayoutNode{
		Type:      tmux.LayoutSplit,
		Direction: tmux.SplitHorizontal,
		Ratio:     0.5,
		Children: [2]*tmux.LayoutNode{
			{Type: tmux.LayoutLeaf, PaneID: 0},
			{Type: tmux.LayoutLeaf, PaneID: 1},
		},
	})})

	if !changed {
		t.Error("layout change should be detected")
	}
}

func TestLayoutSnapshotEqualDetectsStructuralChanges(t *testing.T) {
	leaf := func(id int) *tmux.LayoutNode {
		return &tmux.LayoutNode{Type: tmux.LayoutLeaf, PaneID: id}
	}
	split := func(dir tmux.SplitDirection, ratio float64, l, r *tmux.LayoutNode) *tmux.LayoutNode {
		return &tmux.LayoutNode{
			Type:      tmux.LayoutSplit,
			Direction: dir,
			Ratio:     ratio,
			Children:  [2]*tmux.LayoutNode{l, r},
		}
	}

	tests := []struct {
		name  string
		left  *tmux.LayoutNode
		right *tmux.LayoutNode
		want  bool
	}{
		{"both_nil", nil, nil, true},
		{"left_nil", nil, leaf(0), false},
		{"right_nil", leaf(0), nil, false},
		{"same_leaf", leaf(0), leaf(0), true},
		{"diff_pane_id", leaf(0), leaf(1), false},
		{"type_mismatch", leaf(0), split(tmux.SplitHorizontal, 0.5, leaf(0), leaf(1)), false},
		{"direction_change",
			split(tmux.SplitHorizontal, 0.5, leaf(0), leaf(1)),
			split(tmux.SplitVertical, 0.5, leaf(0), leaf(1)),
			false,
		},
		{"ratio_change",
			split(tmux.SplitHorizontal, 0.5, leaf(0), leaf(1)),
			split(tmux.SplitHorizontal, 0.7, leaf(0), leaf(1)),
			false,
		},
		{"identical_split",
			split(tmux.SplitHorizontal, 0.5, leaf(0), leaf(1)),
			split(tmux.SplitHorizontal, 0.5, leaf(0), leaf(1)),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := layoutSnapshotEqual(tt.left, tt.right)
			if got != tt.want {
				t.Errorf("layoutSnapshotEqual = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RootPath / Worktree change detection
// ---------------------------------------------------------------------------

func TestSnapshotDeltaDetectsRootPathChange(t *testing.T) {
	svc := newTestService(t)

	base := testSnapshot("s1", 1, false)
	base.RootPath = "/old/path"
	svc.snapshotDelta([]tmux.SessionSnapshot{base})

	modified := testSnapshot("s1", 1, false)
	modified.RootPath = "/new/path"
	_, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{modified})
	if !changed {
		t.Error("RootPath change should be detected")
	}
}

func TestSnapshotDeltaDetectsWorktreeChange(t *testing.T) {
	svc := newTestService(t)

	base := testSnapshot("s1", 1, false)
	base.Worktree = &tmux.SessionWorktreeInfo{Path: "/wt", BranchName: "main"}
	svc.snapshotDelta([]tmux.SessionSnapshot{base})

	modified := testSnapshot("s1", 1, false)
	modified.Worktree = &tmux.SessionWorktreeInfo{Path: "/wt", BranchName: "feature"}
	_, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{modified})
	if !changed {
		t.Error("Worktree.BranchName change should be detected")
	}
}

func TestSnapshotDeltaDetectsWorktreeNilTransitions(t *testing.T) {
	tests := []struct {
		name  string
		left  *tmux.SessionWorktreeInfo
		right *tmux.SessionWorktreeInfo
	}{
		{"nil_to_non_nil", nil, &tmux.SessionWorktreeInfo{Path: "/wt"}},
		{"non_nil_to_nil", &tmux.SessionWorktreeInfo{Path: "/wt"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)

			base := testSnapshot("s1", 1, false)
			base.Worktree = tt.left
			svc.snapshotDelta([]tmux.SessionSnapshot{base})

			modified := testSnapshot("s1", 1, false)
			modified.Worktree = tt.right
			_, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{modified})
			if !changed {
				t.Error("nil/non-nil worktree transition should be detected")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Sequential operations: add -> remove -> add back -> no change
// ---------------------------------------------------------------------------

func TestSnapshotDeltaSequentialOperations(t *testing.T) {
	svc := newTestService(t)

	// Step 1: seed with s1.
	svc.snapshotDelta([]tmux.SessionSnapshot{testSnapshot("s1", 1, false)})

	// Step 2: add s2.
	delta, changed, _ := svc.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
		testSnapshot("s2", 2, false),
	})
	if !changed {
		t.Fatal("adding s2 should be detected")
	}
	if len(delta.Upserts) != 1 || delta.Upserts[0].Name != "s2" {
		t.Errorf("step 2: upserts = %v", delta.Upserts)
	}

	// Step 3: remove s2.
	delta, changed, _ = svc.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
	})
	if !changed {
		t.Fatal("removing s2 should be detected")
	}
	if len(delta.Removed) != 1 || delta.Removed[0] != "s2" {
		t.Errorf("step 3: removed = %v", delta.Removed)
	}

	// Step 4: add s2 back.
	delta, changed, _ = svc.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
		testSnapshot("s2", 2, false),
	})
	if !changed {
		t.Fatal("adding s2 back should be detected")
	}
	if len(delta.Upserts) != 1 || delta.Upserts[0].Name != "s2" {
		t.Errorf("step 4: upserts = %v", delta.Upserts)
	}

	// Step 5: no change.
	_, changed, _ = svc.snapshotDelta([]tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
		testSnapshot("s2", 2, false),
	})
	if changed {
		t.Error("step 5: identical snapshot should report no change")
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestSnapshotDeltaConcurrentCallsRemainStable(t *testing.T) {
	svc := newTestService(t)

	// Seed.
	svc.snapshotDelta([]tmux.SessionSnapshot{testSnapshot("s1", 1, false)})

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	snapshots := []tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
		testSnapshot("s2", 2, true),
	}
	for range goroutines {
		go func() {
			defer wg.Done()
			// Must not panic or corrupt cache.
			svc.snapshotDelta(snapshots)
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Benchmarks (Go 1.26 b.Loop pattern)
// ---------------------------------------------------------------------------

func BenchmarkSnapshotDelta(b *testing.B) {
	svc := newTestService(&testing.T{})

	snapshots := make([]tmux.SessionSnapshot, 5)
	for i := range snapshots {
		now := time.Now()
		snapshots[i] = tmux.SessionSnapshot{
			Name:      "session-" + time.Now().Format("150405"),
			ID:        i,
			CreatedAt: now,
			Windows: []tmux.WindowSnapshot{
				{
					ID:   i,
					Name: "win",
					Panes: []tmux.PaneSnapshot{
						{ID: "%0", Index: 0, Active: true, Width: 120, Height: 40},
					},
					Layout: &tmux.LayoutNode{Type: tmux.LayoutLeaf, PaneID: 0},
				},
			},
		}
	}

	// Seed.
	svc.snapshotDelta(snapshots)

	for b.Loop() {
		svc.snapshotDelta(snapshots)
	}
}

func BenchmarkSnapshotDelta_NoChange(b *testing.B) {
	svc := newTestService(&testing.T{})
	snap := []tmux.SessionSnapshot{testSnapshot("s1", 1, false)}
	svc.snapshotDelta(snap)

	for b.Loop() {
		svc.snapshotDelta(snap)
	}
}

func BenchmarkSnapshotDelta_OneSessionChanged(b *testing.B) {
	svc := newTestService(&testing.T{})

	base := []tmux.SessionSnapshot{
		testSnapshot("s1", 1, false),
		testSnapshot("s2", 2, false),
		testSnapshot("s3", 3, false),
	}
	svc.snapshotDelta(base)

	changed := make([]tmux.SessionSnapshot, len(base))
	copy(changed, base)
	changed[1] = testSnapshot("s2", 2, true) // IsIdle flipped

	toggle := false
	for b.Loop() {
		if toggle {
			svc.snapshotDelta(base)
		} else {
			svc.snapshotDelta(changed)
		}
		toggle = !toggle
	}
}
