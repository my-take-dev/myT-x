package tmux

import (
	"fmt"
	"sort"
)

// Snapshot returns deep-copied frontend-safe session state.
func (m *SessionManager) Snapshot() []SessionSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return m.sessions[names[i]].ID < m.sessions[names[j]].ID
	})

	out := make([]SessionSnapshot, 0, len(names))
	for _, name := range names {
		session := m.sessions[name]
		var worktree *SessionWorktreeInfo
		if session.Worktree != nil {
			copied := *session.Worktree
			worktree = &copied
		}
		ss := SessionSnapshot{
			ID:          session.ID,
			Name:        session.Name,
			CreatedAt:   session.CreatedAt,
			IsIdle:      session.IsIdle,
			IsAgentTeam: session.IsAgentTeam,
			Windows:     make([]WindowSnapshot, 0, len(session.Windows)),
			Worktree:    worktree,
			RootPath:    session.RootPath,
		}
		for _, window := range session.Windows {
			ws := WindowSnapshot{
				ID:       window.ID,
				Name:     window.Name,
				Layout:   cloneLayout(window.Layout),
				ActivePN: window.ActivePN,
				Panes:    make([]PaneSnapshot, 0, len(window.Panes)),
			}
			for _, pane := range window.Panes {
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

	return out
}

// ActivePaneIDs returns the set of all pane ID strings currently managed.
// This is a lightweight alternative to Snapshot() when only pane IDs are needed.
func (m *SessionManager) ActivePaneIDs() map[string]struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make(map[string]struct{}, len(m.panes))
	for id := range m.panes {
		ids[fmt.Sprintf("%%%d", id)] = struct{}{}
	}
	return ids
}
