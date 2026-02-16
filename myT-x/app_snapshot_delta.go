package main

import (
	"sort"

	"myT-x/internal/tmux"
)

// sessionSnapshotEqual performs field-by-field equality comparison, replacing
// reflect.DeepEqual to avoid reflection overhead on the snapshot emission hot path.
// IMPORTANT: update this function when fields are added/removed from SessionSnapshot.
// TestSnapshotFieldCounts guards against forgetting this via reflection-based field count checks.
func sessionSnapshotEqual(left, right tmux.SessionSnapshot) bool {
	if left.ID != right.ID {
		return false
	}
	if left.Name != right.Name {
		return false
	}
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return false
	}
	if left.IsIdle != right.IsIdle {
		return false
	}
	if left.IsAgentTeam != right.IsAgentTeam {
		return false
	}
	if len(left.Windows) != len(right.Windows) {
		return false
	}
	for idx := range left.Windows {
		if !windowSnapshotEqual(left.Windows[idx], right.Windows[idx]) {
			return false
		}
	}
	if !sessionWorktreeInfoEqual(left.Worktree, right.Worktree) {
		return false
	}
	if left.RootPath != right.RootPath {
		return false
	}
	return true
}

func sessionWorktreeInfoEqual(left, right *tmux.SessionWorktreeInfo) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Path != right.Path {
		return false
	}
	if left.RepoPath != right.RepoPath {
		return false
	}
	if left.BranchName != right.BranchName {
		return false
	}
	if left.BaseBranch != right.BaseBranch {
		return false
	}
	if left.IsDetached != right.IsDetached {
		return false
	}
	return true
}

func windowSnapshotEqual(left, right tmux.WindowSnapshot) bool {
	if left.ID != right.ID {
		return false
	}
	if left.Name != right.Name {
		return false
	}
	if left.ActivePN != right.ActivePN {
		return false
	}
	if !layoutSnapshotEqual(left.Layout, right.Layout) {
		return false
	}
	if len(left.Panes) != len(right.Panes) {
		return false
	}
	for idx := range left.Panes {
		if !paneSnapshotEqual(left.Panes[idx], right.Panes[idx]) {
			return false
		}
	}
	return true
}

func paneSnapshotEqual(left, right tmux.PaneSnapshot) bool {
	if left.ID != right.ID {
		return false
	}
	if left.Index != right.Index {
		return false
	}
	if left.Title != right.Title {
		return false
	}
	if left.Active != right.Active {
		return false
	}
	if left.Width != right.Width {
		return false
	}
	if left.Height != right.Height {
		return false
	}
	return true
}

func layoutSnapshotEqual(left, right *tmux.LayoutNode) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Type != right.Type {
		return false
	}
	if left.Direction != right.Direction {
		return false
	}
	if left.Ratio != right.Ratio {
		return false
	}
	if left.PaneID != right.PaneID {
		return false
	}
	// Children is [2]*LayoutNode; leaf nodes have nil entries handled above.
	return layoutSnapshotEqual(left.Children[0], right.Children[0]) &&
		layoutSnapshotEqual(left.Children[1], right.Children[1])
}

func (a *App) snapshotDelta(snapshots []tmux.SessionSnapshot) (tmux.SessionSnapshotDelta, bool, bool) {
	a.snapshotDeltaMu.Lock()
	defer a.snapshotDeltaMu.Unlock()

	a.snapshotMu.Lock()
	if !a.snapshotPrimed {
		if a.snapshotCache == nil {
			a.snapshotCache = make(map[string]tmux.SessionSnapshot, len(snapshots))
		}
		for _, snapshot := range snapshots {
			a.snapshotCache[snapshot.Name] = snapshot
		}
		a.snapshotPrimed = true
		a.snapshotMu.Unlock()
		return tmux.SessionSnapshotDelta{}, false, true
	}
	// NOTE: snapshotDelta intentionally computes outside snapshotMu to avoid
	// holding the cache lock across full snapshot comparison on the hot path.
	// snapshotDeltaMu serializes emit paths so cache updates cannot overwrite each
	// other when requestSnapshot(true) is triggered concurrently.
	previous := copySnapshotCache(a.snapshotCache)
	a.snapshotMu.Unlock()

	delta := tmux.SessionSnapshotDelta{
		Upserts: make([]tmux.SessionSnapshot, 0, len(snapshots)),
		Removed: make([]string, 0),
	}

	// Build a lightweight name set for removal detection.
	currentNames := make(map[string]struct{}, len(snapshots))
	for _, snapshot := range snapshots {
		currentNames[snapshot.Name] = struct{}{}
		prev, ok := previous[snapshot.Name]
		if ok && sessionSnapshotEqual(prev, snapshot) {
			continue
		}
		delta.Upserts = append(delta.Upserts, snapshot)
	}

	for name := range previous {
		if _, ok := currentNames[name]; ok {
			continue
		}
		delta.Removed = append(delta.Removed, name)
	}
	if len(delta.Removed) > 1 {
		sort.Strings(delta.Removed)
	}

	// Update cache in-place on the copied map: remove stale, then upsert changes.
	for _, name := range delta.Removed {
		delete(previous, name)
	}
	for _, snapshot := range delta.Upserts {
		previous[snapshot.Name] = snapshot
	}

	a.snapshotMu.Lock()
	if !a.snapshotPrimed || a.snapshotCache == nil {
		a.snapshotCache = make(map[string]tmux.SessionSnapshot, len(snapshots))
		for _, snapshot := range snapshots {
			a.snapshotCache[snapshot.Name] = snapshot
		}
		a.snapshotPrimed = true
		a.snapshotMu.Unlock()
		return tmux.SessionSnapshotDelta{}, false, true
	}
	a.snapshotCache = previous
	a.snapshotMu.Unlock()

	return delta, len(delta.Upserts) > 0 || len(delta.Removed) > 0, false
}

func copySnapshotCache(cache map[string]tmux.SessionSnapshot) map[string]tmux.SessionSnapshot {
	out := make(map[string]tmux.SessionSnapshot, len(cache))
	for name, snapshot := range cache {
		out[name] = snapshot
	}
	return out
}
