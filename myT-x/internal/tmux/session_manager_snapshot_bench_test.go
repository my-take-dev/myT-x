package tmux

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkSnapshotClone measures deep-clone cost for a realistic snapshot
// topology: 10 sessions x 5 windows x 4 panes = 200 panes total.
// This represents the upper bound of typical production deployments.
//
// Performance note: cloneSessionSnapshots for 10 sessions x 5 windows x 4 panes
// benchmarks at ~11 us / 27 KiB alloc (2026-02-20, i7-13620H). Well under the
// 50 ms snapshot coalesce window; a read-only SnapshotReadOnly() path is not
// warranted at this scale.
func BenchmarkSnapshotClone(b *testing.B) {
	snapshots := buildBenchmarkSnapshots(10, 5, 4)

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_ = cloneSessionSnapshots(snapshots)
	}
}

// BenchmarkSnapshotClone_Small measures clone cost for a minimal topology:
// 1 session x 1 window x 1 pane. This represents the lower bound.
func BenchmarkSnapshotClone_Small(b *testing.B) {
	snapshots := buildBenchmarkSnapshots(1, 1, 1)

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_ = cloneSessionSnapshots(snapshots)
	}
}

// buildBenchmarkSnapshots constructs a synthetic snapshot slice for benchmarking.
// Parameters control the topology size: sessions x windows x panes.
func buildBenchmarkSnapshots(numSessions, numWindows, numPanes int) []SessionSnapshot {
	snapshots := make([]SessionSnapshot, numSessions)
	for i := range snapshots {
		windows := make([]WindowSnapshot, numWindows)
		for j := range windows {
			panes := make([]PaneSnapshot, numPanes)
			for k := range panes {
				panes[k] = PaneSnapshot{
					ID:     fmt.Sprintf("%%%d", k+j*numPanes+i*numWindows*numPanes),
					Index:  k,
					Width:  120,
					Height: 40,
					Title:  fmt.Sprintf("pane-%d-%d-%d", i, j, k),
					Active: k == 0,
				}
			}
			// Build a simple leaf layout for each pane to exercise cloneLayout.
			var layout *LayoutNode
			if numPanes == 1 {
				layout = newLeafLayout(firstPaneIDInWindow(i, j, numWindows, numPanes))
			} else {
				layout = buildBenchLayout(i, j, numPanes, numWindows)
			}
			windows[j] = WindowSnapshot{
				ID:       j + i*numWindows,
				Name:     fmt.Sprintf("window-%d-%d", i, j),
				Layout:   layout,
				ActivePN: 0,
				Panes:    panes,
			}
		}

		var worktree *SessionWorktreeInfo
		// Assign worktree metadata to even-numbered sessions to exercise
		// the Worktree pointer clone path.
		if i%2 == 0 {
			worktree = &SessionWorktreeInfo{
				Path:       fmt.Sprintf("/worktrees/session-%d", i),
				RepoPath:   "/repo/root",
				BranchName: fmt.Sprintf("feature/session-%d", i),
				BaseBranch: "main",
			}
		}

		snapshots[i] = SessionSnapshot{
			ID:             i,
			Name:           fmt.Sprintf("session-%d", i),
			CreatedAt:      time.Now(),
			ActiveWindowID: 0,
			Windows:        windows,
			Worktree:       worktree,
			RootPath:       fmt.Sprintf("/projects/session-%d", i),
		}
	}
	return snapshots
}

// firstPaneIDInWindow computes the pane ID for pane index 0 of a given window.
func firstPaneIDInWindow(sessionIdx, windowIdx, numWindows, numPanes int) int {
	return windowIdx*numPanes + sessionIdx*numWindows*numPanes
}

// buildBenchLayout builds a simple binary split layout for benchmarking.
func buildBenchLayout(sessionIdx, windowIdx, numPanes, numWindows int) *LayoutNode {
	if numPanes <= 0 {
		return nil
	}
	if numPanes == 1 {
		return newLeafLayout(firstPaneIDInWindow(sessionIdx, windowIdx, numWindows, numPanes))
	}
	// Build a simple two-leaf split for benchmark purposes.
	return &LayoutNode{
		Type:      LayoutSplit,
		Direction: SplitVertical,
		Ratio:     0.5,
		Children: [2]*LayoutNode{
			newLeafLayout(firstPaneIDInWindow(sessionIdx, windowIdx, numWindows, numPanes)),
			newLeafLayout(firstPaneIDInWindow(sessionIdx, windowIdx, numWindows, numPanes) + 1),
		},
	}
}
