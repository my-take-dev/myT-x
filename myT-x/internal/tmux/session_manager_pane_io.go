package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"myT-x/internal/terminal"
)

// ListPanesByWindowTarget returns panes for a given -t target.
//
// I-4: Returns value copies ([]TmuxPane) instead of internal pointers to
// eliminate data-race risk from accessing *TmuxPane fields after lock release.
// Callers that need to identify panes across lock scopes should use stable
// fields (ID, IDString()) from the returned copies.
//
// TODO(T-23): This method uses independent target resolution logic that
// partially duplicates resolveTargetCore. A future refactoring should unify
// the window-resolution path (bare session name, session:windowIdx,
// session:@windowID) with resolveTargetCore, extracting a shared
// resolveWindowTarget helper. The current implementation is kept because
// resolveTargetCore returns a single pane, whereas this method needs the
// entire window's pane list, making a direct delegation non-trivial.
func (m *SessionManager) ListPanesByWindowTarget(target string, callerPaneID int, allInSession bool) ([]TmuxPane, error) {
	// Lock (not RLock): activeWindowInSessionLocked may auto-repair stale
	// ActiveWindowID when resolving default/implicit active window targets.
	m.mu.Lock()
	defer m.mu.Unlock()

	if allInSession {
		var session *TmuxSession
		var err error
		if strings.TrimSpace(target) == "" && callerPaneID >= 0 {
			pane, ok := m.panes[callerPaneID]
			if !ok || pane == nil || pane.Window == nil || pane.Window.Session == nil {
				return nil, errors.New("caller pane not found")
			}
			session = pane.Window.Session
		} else {
			session, err = m.resolveSessionTargetLocked(target)
			if err != nil {
				return nil, err
			}
		}
		out := make([]TmuxPane, 0)
		for _, window := range session.Windows {
			if window == nil {
				continue
			}
			out = append(out, copyPaneSlice(window.Panes)...)
		}
		return out, nil
	}

	if strings.TrimSpace(target) == "" {
		pane, ok := m.panes[callerPaneID]
		if !ok || pane == nil || pane.Window == nil {
			return nil, errors.New("caller pane not found")
		}
		return copyPaneSlice(pane.Window.Panes), nil
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
			if !ok || pane == nil || pane.Window == nil {
				return nil, fmt.Errorf("pane not found: %s", target)
			}
			return copyPaneSlice(pane.Window.Panes), nil
		}
		session := m.sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("session not found: %s", sessionName)
		}
		activeWindow := m.activeWindowInSessionLocked(session)
		if activeWindow == nil {
			return nil, errors.New("session has no windows")
		}
		return copyPaneSlice(activeWindow.Panes), nil
	}

	session := m.sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}
	if strings.TrimSpace(remainder) == "" {
		activeWindow := m.activeWindowInSessionLocked(session)
		if activeWindow == nil {
			return nil, errors.New("session has no windows")
		}
		return copyPaneSlice(activeWindow.Panes), nil
	}

	windowPart, _, _ := strings.Cut(remainder, ".")

	// I-16: Support @stableID format (e.g., "session:@5") to match
	// resolveWindowPaneTarget behaviour.
	if after, ok := strings.CutPrefix(windowPart, "@"); ok {
		windowIDText := strings.TrimSpace(after)
		windowID, parseErr := strconv.Atoi(windowIDText)
		if parseErr != nil || windowID < 0 {
			return nil, fmt.Errorf("invalid window id: %s", windowPart)
		}
		window, _ := findWindowByID(session.Windows, windowID)
		if window == nil {
			return nil, fmt.Errorf("window id not found: %d", windowID)
		}
		return copyPaneSlice(window.Panes), nil
	}

	windowIdx, err := strconv.Atoi(windowPart)
	if err != nil {
		return nil, fmt.Errorf("invalid window index: %s", windowPart)
	}
	if windowIdx < 0 || windowIdx >= len(session.Windows) {
		return nil, fmt.Errorf("window index out of range: %d", windowIdx)
	}
	// I-05: nil guard for the integer index path. session.Windows[windowIdx]
	// may be nil if the window slice was partially cleared or corrupted.
	window := session.Windows[windowIdx]
	if window == nil {
		return nil, fmt.Errorf("window at index %d is nil", windowIdx)
	}
	return copyPaneSlice(window.Panes), nil
}

// copyPaneSlice creates value copies of panes, skipping nil entries.
// Terminal and Window are explicitly nil-ified in the copies to prevent
// callers from accessing internal state outside of lock scope.
func copyPaneSlice(panes []*TmuxPane) []TmuxPane {
	out := make([]TmuxPane, 0, len(panes))
	for _, pane := range panes {
		if pane == nil {
			continue
		}
		copied := *pane
		copied.Env = copyEnvMap(pane.Env)
		copied.Terminal = nil
		copied.Window = nil
		out = append(out, copied)
	}
	return out
}

// WriteToPane writes input bytes to the given pane id (%N).
func (m *SessionManager) WriteToPane(paneID string, data string) error {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return err
	}

	// Phase 1: resolve terminal pointer under read lock.
	m.mu.RLock()
	pane := m.panes[id]
	if pane == nil || pane.Terminal == nil {
		m.mu.RUnlock()
		return fmt.Errorf("pane not found: %s", paneID)
	}
	term := pane.Terminal
	m.mu.RUnlock()
	// NOTE: Terminal pointer invariant — the Terminal field is set once at pane
	// creation and never replaced. This invariant allows safe use of 'term'
	// after RUnlock. If pane restart or terminal replacement is ever implemented,
	// this assumption must be revisited (consider atomic.Pointer[terminal.Terminal]).
	//
	// Lock-free I/O — Terminal.Write is internally synchronized via Terminal.mu.
	// Releasing SessionManager.mu before the ConPTY syscall prevents reader
	// starvation when multiple panes write concurrently. If the pane is killed
	// between RUnlock and Write, Terminal.Write returns an error ("terminal closed").
	// See defensive-coding-checklist #83.

	// Log writes containing "&&" or "cd " to detect the problematic command path.
	if strings.Contains(data, "&&") || (strings.Contains(data, "cd ") && len(data) > 10) {
		slog.Debug("[DEBUG-WRITE] WriteToPane: suspicious command detected",
			"paneID", paneID,
			"dataLen", len(data),
			"dataPreview", truncateString(data, 300),
		)
	}

	_, err = term.Write([]byte(data))
	return err
}

// WriteToPanesInWindow writes input to all panes in the same window as the specified pane.
func (m *SessionManager) WriteToPanesInWindow(paneID string, data string) error {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return err
	}

	// Phase 1: collect terminal pointers under read lock.
	type termRef struct {
		id   string
		term *terminal.Terminal
	}
	m.mu.RLock()
	pane := m.panes[id]
	if pane == nil || pane.Window == nil {
		m.mu.RUnlock()
		return fmt.Errorf("pane not found: %s", paneID)
	}
	targets := make([]termRef, 0, len(pane.Window.Panes))
	for _, sibling := range pane.Window.Panes {
		if sibling == nil || sibling.Terminal == nil {
			continue
		}
		targets = append(targets, termRef{id: sibling.IDString(), term: sibling.Terminal})
	}
	m.mu.RUnlock()
	// NOTE: Terminal pointer invariant — see WriteToPane comment for the full
	// rationale. Terminal fields are set once at pane creation and never replaced.
	// Each Terminal.Write is internally synchronized via Terminal.mu. Collecting
	// references under lock then writing without lock allows concurrent pane writes.

	var firstErr error
	for _, t := range targets {
		if _, wErr := t.term.Write([]byte(data)); wErr != nil {
			if firstErr == nil {
				firstErr = wErr
			} else {
				// I-27: Log subsequent write errors so they are not silently lost.
				slog.Warn("[DEBUG-PANE] WriteToPanesInWindow: subsequent pane write error",
					"paneId", t.id,
					"error", wErr,
				)
			}
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
	if pane.Width == cols && pane.Height == rows {
		return nil
	}
	if err := pane.Terminal.Resize(cols, rows); err != nil {
		return err
	}
	pane.Width = cols
	pane.Height = rows
	m.markStateMutationLocked()
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
	nextTitle := strings.TrimSpace(title)
	if pane.Title != nextTitle {
		pane.Title = nextTitle
		m.markStateMutationLocked()
	}
	return pane.Window.Session.Name, nil
}
