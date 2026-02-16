package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"myT-x/internal/tmux"
)

type snapshotMetrics struct {
	windowStart  time.Time
	fullCount    int
	deltaCount   int
	fullBytes    int64
	deltaBytes   int64
	fullSamples  int
	deltaSamples int
}

const snapshotMetricsWindow = 10 * time.Second
const snapshotPayloadSampleEvery = 8
const snapshotPayloadNotSampled = -1

// payloadSizeBytes estimates the JSON-serialized byte size of the given snapshot
// or delta payload without performing actual marshaling. The result is an
// approximation that may overcount due to omitempty fields being included.
func payloadSizeBytes(payload any) int {
	switch data := payload.(type) {
	case []tmux.SessionSnapshot:
		return estimateSessionSnapshotListSize(data)
	case tmux.SessionSnapshotDelta:
		return estimateSessionSnapshotDeltaSize(data)
	case *tmux.SessionSnapshotDelta:
		if data == nil {
			return 0
		}
		return estimateSessionSnapshotDeltaSize(*data)
	default:
		slog.Warn("[snapshot-metrics] payloadSizeBytes: unsupported payload type, returning 0",
			"type", fmt.Sprintf("%T", payload))
		return 0
	}
}

func estimateSessionSnapshotDeltaSize(delta tmux.SessionSnapshotDelta) int {
	// {"upserts":[...],"removed":[...]}
	size := 22
	size += estimateSessionSnapshotListSize(delta.Upserts)
	size += 2 // comma separating upserts and removed arrays
	size += estimateStringListSize(delta.Removed)
	return size
}

func estimateSessionSnapshotListSize(snapshots []tmux.SessionSnapshot) int {
	if len(snapshots) == 0 {
		return 2
	}
	size := 2 + (len(snapshots) - 1)
	for _, snapshot := range snapshots {
		size += estimateSessionSnapshotSize(snapshot)
	}
	return size
}

func estimateSessionSnapshotSize(snapshot tmux.SessionSnapshot) int {
	// {"id":...,"name":"...","created_at":"...","is_idle":...,"windows":[...],"worktree":{...}}
	size := 84
	size += estimateIntSize(snapshot.ID)
	size += estimateStringSize(snapshot.Name)
	size += estimateStringSize(snapshot.CreatedAt.Format(time.RFC3339Nano))
	size += estimateBoolSize(snapshot.IsIdle)
	size += estimateBoolSize(snapshot.IsAgentTeam)
	size += estimateWindowSnapshotListSize(snapshot.Windows)
	size += estimateSessionWorktreeInfoSize(snapshot.Worktree)
	size += estimateStringSize(snapshot.RootPath)
	return size
}

func estimateSessionWorktreeInfoSize(worktree *tmux.SessionWorktreeInfo) int {
	if worktree == nil {
		return 0
	}
	// {"path":"...","repo_path":"...","branch_name":"...","base_branch":"...","is_detached":...}
	size := 74
	size += estimateStringSize(worktree.Path)
	size += estimateStringSize(worktree.RepoPath)
	size += estimateStringSize(worktree.BranchName)
	size += estimateStringSize(worktree.BaseBranch)
	size += estimateBoolSize(worktree.IsDetached)
	return size
}

func estimateWindowSnapshotListSize(windows []tmux.WindowSnapshot) int {
	if len(windows) == 0 {
		return 2
	}
	size := 2 + (len(windows) - 1)
	for _, window := range windows {
		size += estimateWindowSnapshotSize(window)
	}
	return size
}

func estimateWindowSnapshotSize(window tmux.WindowSnapshot) int {
	// {"id":...,"name":"...","layout":...,"active_pane":...,"panes":[...]}
	size := 63
	size += estimateIntSize(window.ID)
	size += estimateStringSize(window.Name)
	size += estimateLayoutNodeSize(window.Layout)
	size += estimateIntSize(window.ActivePN)
	size += estimatePaneSnapshotListSize(window.Panes)
	return size
}

func estimatePaneSnapshotListSize(panes []tmux.PaneSnapshot) int {
	if len(panes) == 0 {
		return 2
	}
	size := 2 + (len(panes) - 1)
	for _, pane := range panes {
		size += estimatePaneSnapshotSize(pane)
	}
	return size
}

func estimatePaneSnapshotSize(pane tmux.PaneSnapshot) int {
	// {"id":"...","index":...,"title":"...","active":...,"width":...,"height":...}
	size := 64
	size += estimateStringSize(pane.ID)
	size += estimateIntSize(pane.Index)
	size += estimateStringSize(pane.Title)
	size += estimateBoolSize(pane.Active)
	size += estimateIntSize(pane.Width)
	size += estimateIntSize(pane.Height)
	return size
}

func estimateLayoutNodeSize(node *tmux.LayoutNode) int {
	if node == nil {
		return 4
	}
	// {"type":"...","direction":"...","ratio":...,"pane_id":...,"children":[...]}
	size := 52
	size += estimateStringSize(string(node.Type))
	size += estimateStringSize(string(node.Direction))
	size += len(strconv.FormatFloat(node.Ratio, 'f', -1, 64))
	size += estimateIntSize(node.PaneID)
	size += estimateLayoutNodeSize(node.Children[0])
	size += estimateLayoutNodeSize(node.Children[1])
	return size
}

func estimateStringListSize(values []string) int {
	if len(values) == 0 {
		return 2
	}
	size := 2 + (len(values) - 1)
	for _, value := range values {
		size += estimateStringSize(value)
	}
	return size
}

// estimateStringSize returns a lower-bound estimate; does not account for JSON escape expansion.
func estimateStringSize(value string) int {
	return len(value) + 2
}

func estimateIntSize(value int) int {
	return len(strconv.Itoa(value))
}

func estimateBoolSize(value bool) int {
	if value {
		return 4
	}
	return 5
}

func (a *App) recordSnapshotEmission(kind string, payloadBytes int) {
	hasPayloadSample := payloadBytes >= 0

	a.snapshotMetricsMu.Lock()
	defer a.snapshotMetricsMu.Unlock()

	now := time.Now()
	if a.snapshotStats.windowStart.IsZero() {
		a.snapshotStats.windowStart = now
	}
	switch kind {
	case "full":
		a.snapshotStats.fullCount++
		if hasPayloadSample {
			a.snapshotStats.fullBytes += int64(payloadBytes)
			a.snapshotStats.fullSamples++
		}
	default:
		a.snapshotStats.deltaCount++
		if hasPayloadSample {
			a.snapshotStats.deltaBytes += int64(payloadBytes)
			a.snapshotStats.deltaSamples++
		}
	}

	elapsed := now.Sub(a.snapshotStats.windowStart)
	if elapsed < snapshotMetricsWindow {
		return
	}

	totalCount := a.snapshotStats.fullCount + a.snapshotStats.deltaCount
	totalBytes := a.snapshotStats.fullBytes + a.snapshotStats.deltaBytes
	if totalCount > 0 && elapsed > 0 {
		slog.Info(
			"[snapshot-metrics]",
			"windowMs", elapsed.Milliseconds(),
			"fullCount", a.snapshotStats.fullCount,
			"deltaCount", a.snapshotStats.deltaCount,
			"avgFullBytes", avgBytes(a.snapshotStats.fullBytes, a.snapshotStats.fullSamples),
			"avgDeltaBytes", avgBytes(a.snapshotStats.deltaBytes, a.snapshotStats.deltaSamples),
			"fullSamples", a.snapshotStats.fullSamples,
			"deltaSamples", a.snapshotStats.deltaSamples,
			"eventsPerSec", float64(totalCount)/elapsed.Seconds(),
			"bytesPerSec", float64(totalBytes)/elapsed.Seconds(),
		)
	}

	a.snapshotStats = snapshotMetrics{
		windowStart: now,
	}
}

func (a *App) estimateSnapshotPayloadBytes(payload any) int {
	a.snapshotMetricsMu.Lock()
	eventCount := a.snapshotStats.fullCount + a.snapshotStats.deltaCount
	shouldSample := eventCount%snapshotPayloadSampleEvery == 0
	a.snapshotMetricsMu.Unlock()
	if !shouldSample {
		return snapshotPayloadNotSampled
	}
	return payloadSizeBytes(payload)
}

func avgBytes(bytes int64, count int) int64 {
	if count <= 0 {
		return 0
	}
	return bytes / int64(count)
}
