package main

import (
	"fmt"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// BenchmarkSnapshotDelta_NoChange measures delta computation cost when no
// sessions have changed. This is the steady-state hot path for output-driven
// snapshot emissions where only pane activity triggers re-evaluation.
//
// Performance note: snapshotDelta for 10 sessions x 5 windows x 4 panes with
// no changes benchmarks at ~2.6 us / 4 KiB alloc (2026-02-20, i7-13620H).
// Well under the 50 ms snapshot coalesce window; no optimization needed.
func BenchmarkSnapshotDelta_NoChange(b *testing.B) {
	app := &App{}
	snapshots := buildDeltaBenchSnapshots(10, 5, 4)

	// Prime the delta cache with initial snapshots.
	_, _, initial := app.snapshotDelta(snapshots)
	if !initial {
		b.Fatal("expected initial=true on first call")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_, changed, _ := app.snapshotDelta(snapshots)
		if changed {
			b.Fatal("unexpected change in no-change benchmark")
		}
	}
}

// BenchmarkSnapshotDelta_OneSessionChanged measures delta cost when exactly
// one session out of 10 has changed (IsIdle toggled). This is the common
// case for activity-driven updates.
func BenchmarkSnapshotDelta_OneSessionChanged(b *testing.B) {
	app := &App{}
	snapshots := buildDeltaBenchSnapshots(10, 5, 4)

	// Prime the cache.
	app.snapshotDelta(snapshots)

	// Prepare a modified copy where session-0 has IsIdle toggled.
	modified := make([]tmux.SessionSnapshot, len(snapshots))
	copy(modified, snapshots)
	modified[0].IsIdle = !modified[0].IsIdle

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		// Alternate between modified and original to always produce a delta.
		if n%2 == 0 {
			app.snapshotDelta(modified)
		} else {
			app.snapshotDelta(snapshots)
		}
	}
}

// buildDeltaBenchSnapshots constructs a synthetic snapshot slice for delta benchmarking.
func buildDeltaBenchSnapshots(numSessions, numWindows, numPanes int) []tmux.SessionSnapshot {
	snapshots := make([]tmux.SessionSnapshot, numSessions)
	for i := range snapshots {
		windows := make([]tmux.WindowSnapshot, numWindows)
		for j := range windows {
			panes := make([]tmux.PaneSnapshot, numPanes)
			for k := range panes {
				panes[k] = tmux.PaneSnapshot{
					ID:     fmt.Sprintf("%%%d", k+j*numPanes+i*numWindows*numPanes),
					Index:  k,
					Width:  120,
					Height: 40,
					Title:  fmt.Sprintf("pane-%d-%d-%d", i, j, k),
					Active: k == 0,
				}
			}
			windows[j] = tmux.WindowSnapshot{
				ID:       j + i*numWindows,
				Name:     fmt.Sprintf("window-%d-%d", i, j),
				ActivePN: 0,
				Panes:    panes,
			}
		}
		snapshots[i] = tmux.SessionSnapshot{
			ID:             i,
			Name:           fmt.Sprintf("session-%d", i),
			CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			ActiveWindowID: 0,
			Windows:        windows,
			RootPath:       fmt.Sprintf("/projects/session-%d", i),
		}
	}
	return snapshots
}
