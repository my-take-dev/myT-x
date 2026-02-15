package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// ListPanesByWindowTarget returns panes for a given -t target.
func (m *SessionManager) ListPanesByWindowTarget(target string, callerPaneID int, allInSession bool) ([]*TmuxPane, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if allInSession {
		var session *TmuxSession
		var err error
		if strings.TrimSpace(target) == "" && callerPaneID >= 0 {
			pane, ok := m.panes[callerPaneID]
			if !ok || pane.Window == nil || pane.Window.Session == nil {
				return nil, errors.New("caller pane not found")
			}
			session = pane.Window.Session
		} else {
			session, err = m.resolveSessionTargetLocked(target)
			if err != nil {
				return nil, err
			}
		}
		out := make([]*TmuxPane, 0)
		for _, window := range session.Windows {
			out = append(out, window.Panes...)
		}
		return out, nil
	}

	if strings.TrimSpace(target) == "" {
		pane, ok := m.panes[callerPaneID]
		if !ok || pane.Window == nil {
			return nil, errors.New("caller pane not found")
		}
		return append([]*TmuxPane(nil), pane.Window.Panes...), nil
	}

	sessionName, remainder, hasColon := strings.Cut(target, ":")
	if !hasColon {
		// Pane id (%N) is also supported by ResolveTarget.
		if strings.HasPrefix(target, "%") {
			id, err := parsePaneID(target)
			if err != nil {
				return nil, err
			}
			pane, ok := m.panes[id]
			if !ok || pane.Window == nil {
				return nil, fmt.Errorf("pane not found: %s", target)
			}
			return append([]*TmuxPane(nil), pane.Window.Panes...), nil
		}
		session := m.sessions[sessionName]
		if session == nil || len(session.Windows) == 0 {
			return nil, fmt.Errorf("session not found: %s", sessionName)
		}
		return append([]*TmuxPane(nil), session.Windows[0].Panes...), nil
	}

	session := m.sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}
	if strings.TrimSpace(remainder) == "" {
		if len(session.Windows) == 0 {
			return nil, errors.New("session has no windows")
		}
		return append([]*TmuxPane(nil), session.Windows[0].Panes...), nil
	}

	windowPart, _, _ := strings.Cut(remainder, ".")
	windowIdx, err := strconv.Atoi(windowPart)
	if err != nil {
		return nil, fmt.Errorf("invalid window index: %s", windowPart)
	}
	if windowIdx < 0 || windowIdx >= len(session.Windows) {
		return nil, fmt.Errorf("window index out of range: %d", windowIdx)
	}
	return append([]*TmuxPane(nil), session.Windows[windowIdx].Panes...), nil
}

// WriteToPane writes input bytes to the given pane id (%N).
func (m *SessionManager) WriteToPane(paneID string, data string) error {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	pane := m.panes[id]
	if pane == nil || pane.Terminal == nil {
		return fmt.Errorf("pane not found: %s", paneID)
	}

	// Log writes containing "&&" or "cd " to detect the problematic command path.
	if strings.Contains(data, "&&") || (strings.Contains(data, "cd ") && len(data) > 10) {
		slog.Debug("[DEBUG-WRITE] WriteToPane: suspicious command detected",
			"paneID", paneID,
			"dataLen", len(data),
			"dataPreview", truncateString(data, 300),
		)
	}

	_, err = pane.Terminal.Write([]byte(data))
	return err
}

// WriteToPanesInWindow writes input to all panes in the same window as the specified pane.
func (m *SessionManager) WriteToPanesInWindow(paneID string, data string) error {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	pane := m.panes[id]
	if pane == nil || pane.Window == nil {
		return fmt.Errorf("pane not found: %s", paneID)
	}

	var firstErr error
	for _, sibling := range pane.Window.Panes {
		if sibling.Terminal == nil {
			continue
		}
		if _, wErr := sibling.Terminal.Write([]byte(data)); wErr != nil && firstErr == nil {
			firstErr = wErr
		}
	}
	return firstErr
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ResizePane resizes one pane terminal.
func (m *SessionManager) ResizePane(paneID string, cols, rows int) error {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pane := m.panes[id]
	if pane == nil || pane.Terminal == nil {
		return fmt.Errorf("pane not found: %s", paneID)
	}
	if err := pane.Terminal.Resize(cols, rows); err != nil {
		return err
	}
	pane.Width = cols
	pane.Height = rows
	return nil
}

// RenamePane updates the pane title and returns the owning session name.
func (m *SessionManager) RenamePane(paneID string, title string) (string, error) {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pane := m.panes[id]
	if pane == nil || pane.Window == nil || pane.Window.Session == nil {
		return "", fmt.Errorf("pane not found: %s", paneID)
	}
	pane.Title = strings.TrimSpace(title)
	return pane.Window.Session.Name, nil
}
