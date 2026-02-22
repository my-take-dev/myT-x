package tmux

import (
	"sort"
)

// Snapshot returns frontend-safe session state snapshots.
// Cache miss paths rebuild the cache from internal state.
// All return paths clone the cached slice so callers cannot mutate shared data.
func (m *SessionManager) Snapshot() []SessionSnapshot {
	m.mu.RLock()
	if m.snapshotGeneration == m.generation && m.snapshotCache != nil {
		cached := cloneSessionSnapshots(m.snapshotCache)
		m.mu.RUnlock()
		return cached
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snapshotGeneration == m.generation && m.snapshotCache != nil {
		return cloneSessionSnapshots(m.snapshotCache)
	}

	names := m.sortedSessionNamesLocked()

	out := make([]SessionSnapshot, 0, len(names))
	for _, name := range names {
		session := m.sessions[name]
		var worktree *SessionWorktreeInfo
		if session.Worktree != nil {
			copied := *session.Worktree
			worktree = &copied
		}
		ss := SessionSnapshot{
			ID:             session.ID,
			Name:           session.Name,
			CreatedAt:      session.CreatedAt,
			IsIdle:         session.IsIdle,
			ActiveWindowID: session.ActiveWindowID,
			IsAgentTeam:    session.IsAgentTeam,
			Windows:        make([]WindowSnapshot, 0, len(session.Windows)),
			Worktree:       worktree,
			RootPath:       session.RootPath,
		}
		for _, window := range session.Windows {
			if window == nil {
				continue
			}
			ws := WindowSnapshot{
				ID:       window.ID,
				Name:     window.Name,
				Layout:   cloneLayout(window.Layout),
				ActivePN: window.ActivePN,
				Panes:    make([]PaneSnapshot, 0, len(window.Panes)),
			}
			for _, pane := range window.Panes {
				if pane == nil {
					continue
				}
				ps := PaneSnapshot{
					ID:     pane.IDString(),
					Index:  pane.Index,
					Title:  pane.Title,
					Active: pane.Active,
					Width:  pane.Width,
					Height: pane.Height,
				}
				ws.Panes = append(ws.Panes, ps)
			}
			ss.Windows = append(ss.Windows, ws)
		}
		out = append(out, ss)
	}

	m.snapshotCache = out
	m.snapshotGeneration = m.generation
	return cloneSessionSnapshots(m.snapshotCache)
}

// Clone returns a deep copy of the SessionSnapshot.
// S-1: Extracted from the former cloneSessionSnapshots package-level function
// into a method on SessionSnapshot for better discoverability and testability.
func (ss SessionSnapshot) Clone() SessionSnapshot {
	out := ss

	if ss.Worktree != nil {
		worktreeCopy := *ss.Worktree
		out.Worktree = &worktreeCopy
	}

	if len(ss.Windows) == 0 {
		out.Windows = []WindowSnapshot{}
		return out
	}

	out.Windows = make([]WindowSnapshot, len(ss.Windows))
	for j := range ss.Windows {
		window := ss.Windows[j]
		out.Windows[j] = window
		out.Windows[j].Layout = cloneLayout(window.Layout)

		if len(window.Panes) == 0 {
			out.Windows[j].Panes = []PaneSnapshot{}
			continue
		}
		out.Windows[j].Panes = make([]PaneSnapshot, len(window.Panes))
		copy(out.Windows[j].Panes, window.Panes)
	}
	return out
}

// cloneSessionSnapshots creates independent deep copies of a snapshot slice.
// Delegates to SessionSnapshot.Clone() for each element.
func cloneSessionSnapshots(src []SessionSnapshot) []SessionSnapshot {
	if len(src) == 0 {
		return []SessionSnapshot{}
	}
	out := make([]SessionSnapshot, len(src))
	for i := range src {
		out[i] = src[i].Clone()
	}
	return out
}

func (m *SessionManager) sortedSessionNamesLocked() []string {
	if !m.sortedNamesDirty && len(m.sortedSessionNames) == len(m.sessions) {
		return m.sortedSessionNames
	}
	names := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return m.sessions[names[i]].ID < m.sessions[names[j]].ID
	})
	m.sortedSessionNames = names
	m.sortedNamesDirty = false
	return m.sortedSessionNames
}

// ActivePaneIDs returns the set of all pane ID strings currently managed.
// This is a lightweight alternative to Snapshot() when only pane IDs are needed.
func (m *SessionManager) ActivePaneIDs() map[string]struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make(map[string]struct{}, len(m.panes))
	for _, pane := range m.panes {
		if pane == nil {
			continue
		}
		ids[pane.IDString()] = struct{}{}
	}
	return ids
}
