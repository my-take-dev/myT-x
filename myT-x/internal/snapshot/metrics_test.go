package snapshot

import (
	"reflect"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// PayloadSizeBytes
// ---------------------------------------------------------------------------

func TestPayloadSizeBytes(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		wantGt  int // result must be > wantGt
	}{
		{
			name:    "empty_session_list",
			payload: []tmux.SessionSnapshot{},
			wantGt:  0, // at minimum "[]" = 2 bytes
		},
		{
			name: "single_session",
			payload: []tmux.SessionSnapshot{
				{
					Name:      "test",
					ID:        1,
					CreatedAt: time.Now(),
					Windows: []tmux.WindowSnapshot{
						{
							ID:   1,
							Name: "main",
							Panes: []tmux.PaneSnapshot{
								{ID: "%0", Index: 0, Active: true, Width: 80, Height: 24},
							},
						},
					},
				},
			},
			wantGt: 100,
		},
		{
			name: "unsupported_type",
			payload: struct {
				X int
			}{X: 42},
			wantGt: -1, // returns 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PayloadSizeBytes(tt.payload)
			if got <= tt.wantGt && tt.name != "unsupported_type" {
				t.Errorf("PayloadSizeBytes() = %d, want > %d", got, tt.wantGt)
			}
			if tt.name == "unsupported_type" && got != 0 {
				t.Errorf("PayloadSizeBytes(unsupported) = %d, want 0", got)
			}
		})
	}
}

func TestPayloadSizeBytesDelta(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		wantGt  int
	}{
		{
			name: "delta_value",
			payload: tmux.SessionSnapshotDelta{
				Upserts: []tmux.SessionSnapshot{{Name: "s1", ID: 1}},
				Removed: []string{"s2"},
			},
			wantGt: 20,
		},
		{
			name: "delta_pointer",
			payload: &tmux.SessionSnapshotDelta{
				Upserts: []tmux.SessionSnapshot{{Name: "s1", ID: 1}},
			},
			wantGt: 20,
		},
		{
			name:    "delta_nil_pointer",
			payload: (*tmux.SessionSnapshotDelta)(nil),
			wantGt:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PayloadSizeBytes(tt.payload)
			if tt.name == "delta_nil_pointer" {
				if got != 0 {
					t.Errorf("PayloadSizeBytes(nil delta ptr) = %d, want 0", got)
				}
				return
			}
			if got <= tt.wantGt {
				t.Errorf("PayloadSizeBytes() = %d, want > %d", got, tt.wantGt)
			}
		})
	}
}

func TestPayloadSizeBytesPointerAndDefault(t *testing.T) {
	// Zero-value delta via pointer.
	delta := &tmux.SessionSnapshotDelta{}
	got := PayloadSizeBytes(delta)
	if got <= 0 {
		t.Errorf("expected positive size for empty delta, got %d", got)
	}

	// Unsupported type returns 0.
	if PayloadSizeBytes(42) != 0 {
		t.Errorf("expected 0 for int payload")
	}
}

// ---------------------------------------------------------------------------
// estimateSessionWorktreeInfoSize
// ---------------------------------------------------------------------------

func TestEstimateSessionWorktreeInfoSizeNilAndNonNil(t *testing.T) {
	nilSize := estimateSessionWorktreeInfoSize(nil)
	if nilSize != 0 {
		t.Errorf("nil worktree size = %d, want 0", nilSize)
	}

	nonNilSize := estimateSessionWorktreeInfoSize(&tmux.SessionWorktreeInfo{
		Path:       "/path/to/worktree",
		RepoPath:   "/path/to/repo",
		BranchName: "feature-branch",
		BaseBranch: "main",
		IsDetached: false,
	})
	if nonNilSize <= 0 {
		t.Error("non-nil worktree should have positive size")
	}
}

func TestPayloadSizeBytesIncreasesWhenWorktreeIsPresent(t *testing.T) {
	withoutWT := []tmux.SessionSnapshot{{Name: "s1", ID: 1}}
	withWT := []tmux.SessionSnapshot{{
		Name: "s1", ID: 1,
		Worktree: &tmux.SessionWorktreeInfo{
			Path:       "/wt",
			BranchName: "main",
		},
	}}

	sizeWithout := PayloadSizeBytes(withoutWT)
	sizeWith := PayloadSizeBytes(withWT)

	if sizeWith <= sizeWithout {
		t.Errorf("worktree should increase size: without=%d, with=%d", sizeWithout, sizeWith)
	}
}

// ---------------------------------------------------------------------------
// snapshotMetrics field count guard
// ---------------------------------------------------------------------------

func TestSnapshotMetricsFieldCounts(t *testing.T) {
	const wantFields = 7
	got := reflect.TypeFor[snapshotMetrics]().NumField()
	if got != wantFields {
		t.Errorf("snapshotMetrics has %d fields, want %d; update metrics logic and this test", got, wantFields)
	}
}

// ---------------------------------------------------------------------------
// avgBytes
// ---------------------------------------------------------------------------

func TestAvgBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		count int
		want  int64
	}{
		{"zero_count", 100, 0, 0},
		{"negative_count", 100, -1, 0},
		{"normal", 100, 10, 10},
		{"exact_division", 30, 3, 10},
		{"truncated_division", 100, 3, 33},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := avgBytes(tt.bytes, tt.count)
			if got != tt.want {
				t.Errorf("avgBytes(%d, %d) = %d, want %d", tt.bytes, tt.count, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// recordSnapshotEmission (via Service)
// ---------------------------------------------------------------------------

func TestRecordSnapshotEmissionRollsWindow(t *testing.T) {
	svc := newTestService(t)

	// Force the metrics window to something very small for testing.
	svc.snapshotMetricsMu.Lock()
	svc.snapshotStats.windowStart = time.Now().Add(-snapshotMetricsWindow - time.Second)
	svc.snapshotMetricsMu.Unlock()

	payload := []tmux.SessionSnapshot{{Name: "s1", ID: 1}}
	svc.recordSnapshotEmission("full", payload)

	svc.snapshotMetricsMu.Lock()
	defer svc.snapshotMetricsMu.Unlock()

	// After the window elapses, stats should be reset (windowStart updated, counts zeroed).
	if svc.snapshotStats.fullCount != 0 {
		t.Errorf("fullCount should be 0 after window roll, got %d", svc.snapshotStats.fullCount)
	}
}

func TestRecordSnapshotEmissionCountsFullAndDelta(t *testing.T) {
	svc := newTestService(t)

	payload := []tmux.SessionSnapshot{{Name: "s1", ID: 1}}
	svc.recordSnapshotEmission("full", payload)  // eventCount=0 → sampled
	svc.recordSnapshotEmission("full", payload)  // eventCount=1 → not sampled
	svc.recordSnapshotEmission("delta", payload) // eventCount=2 → not sampled

	svc.snapshotMetricsMu.Lock()
	defer svc.snapshotMetricsMu.Unlock()

	if svc.snapshotStats.fullCount != 2 {
		t.Errorf("fullCount = %d, want 2", svc.snapshotStats.fullCount)
	}
	if svc.snapshotStats.deltaCount != 1 {
		t.Errorf("deltaCount = %d, want 1", svc.snapshotStats.deltaCount)
	}
	// Only the first emission is sampled (eventCount=0, 0%8==0).
	if svc.snapshotStats.fullSamples != 1 {
		t.Errorf("fullSamples = %d, want 1", svc.snapshotStats.fullSamples)
	}
	if svc.snapshotStats.deltaSamples != 0 {
		t.Errorf("deltaSamples = %d, want 0", svc.snapshotStats.deltaSamples)
	}
	if svc.snapshotStats.fullBytes <= 0 {
		t.Error("fullBytes should be positive for the sampled emission")
	}
}

// ---------------------------------------------------------------------------
// recordSnapshotEmission sampling (integrated test)
// ---------------------------------------------------------------------------

func TestRecordSnapshotEmissionSampling(t *testing.T) {
	svc := newTestService(t)
	payload := []tmux.SessionSnapshot{{Name: "s1", ID: 1}}

	// Record 16 emissions and verify sampling rate.
	for range 16 {
		svc.recordSnapshotEmission("full", payload)
	}

	svc.snapshotMetricsMu.Lock()
	defer svc.snapshotMetricsMu.Unlock()

	if svc.snapshotStats.fullCount != 16 {
		t.Errorf("fullCount = %d, want 16", svc.snapshotStats.fullCount)
	}
	// With snapshotPayloadSampleEvery=8:
	// Sampled at eventCount 0 (before 1st emit) and 8 (before 9th emit).
	if svc.snapshotStats.fullSamples != 2 {
		t.Errorf("fullSamples = %d, want 2 (sampled at eventCount 0 and 8)", svc.snapshotStats.fullSamples)
	}
}
