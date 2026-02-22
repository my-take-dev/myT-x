package main

import (
	"reflect"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

func TestPayloadSizeBytes(t *testing.T) {
	snapshots := []tmux.SessionSnapshot{
		{
			ID:        1,
			Name:      "alpha",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Windows: []tmux.WindowSnapshot{
				{
					ID:       0,
					Name:     "0",
					ActivePN: 0,
					Panes: []tmux.PaneSnapshot{
						{ID: "%0", Index: 0, Active: true, Width: 120, Height: 40},
					},
				},
			},
		},
	}
	size := payloadSizeBytes(snapshots)
	if size <= 0 {
		t.Fatalf("payloadSizeBytes() = %d, want > 0", size)
	}
}

func TestPayloadSizeBytesDelta(t *testing.T) {
	size := payloadSizeBytes(tmux.SessionSnapshotDelta{
		Upserts: []tmux.SessionSnapshot{
			{ID: 1, Name: "alpha", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		Removed: []string{"beta"},
	})
	if size <= 0 {
		t.Fatalf("payloadSizeBytes(delta) = %d, want > 0", size)
	}
}

func TestRecordSnapshotEmissionRollsWindow(t *testing.T) {
	app := NewApp()
	app.snapshotMetricsMu.Lock()
	app.snapshotStats.windowStart = time.Unix(0, 0)
	app.snapshotMetricsMu.Unlock()

	app.recordSnapshotEmission("delta", 128)

	app.snapshotMetricsMu.Lock()
	defer app.snapshotMetricsMu.Unlock()
	if app.snapshotStats.deltaCount != 0 || app.snapshotStats.fullCount != 0 {
		t.Fatalf("snapshot stats should be reset after report: %#v", app.snapshotStats)
	}
	if app.snapshotStats.windowStart.IsZero() {
		t.Fatal("windowStart should be initialized after reset")
	}
}

func TestRecordSnapshotEmissionCountsFullPayloads(t *testing.T) {
	app := NewApp()
	app.snapshotMetricsMu.Lock()
	app.snapshotStats.windowStart = time.Now()
	app.snapshotMetricsMu.Unlock()

	app.recordSnapshotEmission("full", 256)

	app.snapshotMetricsMu.Lock()
	defer app.snapshotMetricsMu.Unlock()
	if app.snapshotStats.fullCount != 1 {
		t.Fatalf("fullCount = %d, want 1", app.snapshotStats.fullCount)
	}
	if app.snapshotStats.fullBytes != 256 {
		t.Fatalf("fullBytes = %d, want 256", app.snapshotStats.fullBytes)
	}
	if app.snapshotStats.deltaCount != 0 || app.snapshotStats.deltaBytes != 0 {
		t.Fatalf("delta stats should remain zero, got count=%d bytes=%d", app.snapshotStats.deltaCount, app.snapshotStats.deltaBytes)
	}
}

func TestSnapshotMetricsFieldCounts(t *testing.T) {
	tests := []struct {
		name     string
		numField int
		expected int
	}{
		{"SessionSnapshot", reflect.TypeFor[tmux.SessionSnapshot]().NumField(), 9},
		{"SessionWorktreeInfo", reflect.TypeFor[tmux.SessionWorktreeInfo]().NumField(), 5},
		{"WindowSnapshot", reflect.TypeFor[tmux.WindowSnapshot]().NumField(), 5},
		{"PaneSnapshot", reflect.TypeFor[tmux.PaneSnapshot]().NumField(), 6},
		{"LayoutNode", reflect.TypeFor[tmux.LayoutNode]().NumField(), 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.numField != tt.expected {
				t.Fatalf("%s field count = %d, want %d; update estimate* helpers in app_snapshot_metrics.go", tt.name, tt.numField, tt.expected)
			}
		})
	}
}

func TestPayloadSizeBytesPointerAndDefault(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    int
	}{
		{"nil pointer", (*tmux.SessionSnapshotDelta)(nil), 0},
		{"non-nil pointer", &tmux.SessionSnapshotDelta{
			Upserts: []tmux.SessionSnapshot{{ID: 1, Name: "a", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		}, 0}, // will be > 0, checked below
		{"unknown type returns 0", "some string", 0},
		{"int returns 0", 42, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := payloadSizeBytes(tt.payload)
			switch tt.name {
			case "non-nil pointer":
				if got <= 0 {
					t.Fatalf("payloadSizeBytes(non-nil pointer) = %d, want > 0", got)
				}
				// Verify consistency with value type
				val := *tt.payload.(*tmux.SessionSnapshotDelta)
				valSize := payloadSizeBytes(val)
				if got != valSize {
					t.Fatalf("pointer=%d value=%d, should be equal", got, valSize)
				}
			default:
				if got != tt.want {
					t.Fatalf("payloadSizeBytes(%v) = %d, want %d", tt.payload, got, tt.want)
				}
			}
		})
	}
}

func TestEstimateSessionWorktreeInfoSizeNilAndNonNil(t *testing.T) {
	if got := estimateSessionWorktreeInfoSize(nil); got != 0 {
		t.Fatalf("estimateSessionWorktreeInfoSize(nil) = %d, want 0", got)
	}

	got := estimateSessionWorktreeInfoSize(&tmux.SessionWorktreeInfo{
		Path:       "/repo/.wt/feature-a",
		RepoPath:   "/repo",
		BranchName: "feature-a",
		BaseBranch: "main",
		IsDetached: false,
	})
	if got <= 0 {
		t.Fatalf("estimateSessionWorktreeInfoSize(non-nil) = %d, want > 0", got)
	}
}

func TestPayloadSizeBytesIncreasesWhenWorktreeIsPresent(t *testing.T) {
	base := []tmux.SessionSnapshot{
		{
			ID:        1,
			Name:      "alpha",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Windows:   []tmux.WindowSnapshot{},
		},
	}
	withWorktree := []tmux.SessionSnapshot{
		{
			ID:        1,
			Name:      "alpha",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Windows:   []tmux.WindowSnapshot{},
			Worktree: &tmux.SessionWorktreeInfo{
				Path:       "/repo/.wt/feature-a",
				RepoPath:   "/repo",
				BranchName: "feature-a",
				BaseBranch: "main",
				IsDetached: false,
			},
		},
	}

	baseSize := payloadSizeBytes(base)
	withWorktreeSize := payloadSizeBytes(withWorktree)
	if withWorktreeSize <= baseSize {
		t.Fatalf("payload with worktree size = %d, base size = %d; want worktree > base", withWorktreeSize, baseSize)
	}
}

func TestAvgBytes(t *testing.T) {
	if got := avgBytes(10, 0); got != 0 {
		t.Fatalf("avgBytes(10,0) = %d, want 0", got)
	}
	if got := avgBytes(21, 2); got != 10 {
		t.Fatalf("avgBytes(21,2) = %d, want 10", got)
	}
}

func TestEstimateSnapshotPayloadBytesSampling(t *testing.T) {
	app := NewApp()
	payload := []tmux.SessionSnapshot{
		{
			ID:        1,
			Name:      "alpha",
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	app.snapshotMetricsMu.Lock()
	app.snapshotStats.fullCount = 1
	app.snapshotStats.deltaCount = 0
	app.snapshotMetricsMu.Unlock()
	if got := app.estimateSnapshotPayloadBytes(payload); got != snapshotPayloadNotSampled {
		t.Fatalf("estimateSnapshotPayloadBytes() = %d, want %d", got, snapshotPayloadNotSampled)
	}

	app.snapshotMetricsMu.Lock()
	app.snapshotStats.fullCount = snapshotPayloadSampleEvery
	app.snapshotStats.deltaCount = 0
	app.snapshotMetricsMu.Unlock()
	if got := app.estimateSnapshotPayloadBytes(payload); got <= 0 {
		t.Fatalf("estimateSnapshotPayloadBytes() = %d, want > 0 when sampled", got)
	}
}
