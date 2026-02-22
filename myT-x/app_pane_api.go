package main

import (
	"errors"
	"log/slog"
	"strings"

	"myT-x/internal/tmux"
)

// SplitPane splits one pane. horizontal=true means left/right split.
// NOTE: Unlike other pane API methods, SplitPane delegates to CommandRouter
// (not SessionManager directly), so requireSessionsWithPaneID is not used.
// Validation follows the same TrimSpace + empty check pattern manually.
func (a *App) SplitPane(paneID string, horizontal bool) (string, error) {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return "", errors.New("pane id is required")
	}
	router, err := a.requireRouter()
	if err != nil {
		return "", err
	}

	newPaneID, err := router.SplitWindowInternal(paneID, horizontal)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(newPaneID), nil
}

// SendInput writes raw input bytes to a pane.
func (a *App) SendInput(paneID string, input string) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}
	// Keep input untrimmed to preserve intentional whitespace/newline payloads.
	if err := sessions.WriteToPane(paneID, input); err != nil {
		slog.Debug("[PANE] SendInput failed", "paneID", paneID, "err", err)
		return err
	}
	return nil
}

// SendSyncInput writes input to all panes in the same window as the given pane.
func (a *App) SendSyncInput(paneID string, input string) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}
	if err := sessions.WriteToPanesInWindow(paneID, input); err != nil {
		slog.Debug("[PANE] SendSyncInput failed", "paneID", paneID, "err", err)
		return err
	}
	return nil
}

// GetPaneReplay returns buffered output for a pane to restore terminal view.
func (a *App) GetPaneReplay(paneID string) string {
	if a.paneStates == nil {
		return ""
	}
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return ""
	}
	return a.paneStates.Snapshot(paneID)
}

// ResizePane updates pane PTY size.
func (a *App) ResizePane(paneID string, cols int, rows int) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}
	if err := sessions.ResizePane(paneID, cols, rows); err != nil {
		return err
	}
	if a.paneStates != nil {
		a.paneStates.ResizePane(paneID, cols, rows)
	}
	return nil
}

// FocusPane selects pane as active.
func (a *App) FocusPane(paneID string) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}

	target, err := sessions.ResolveTarget(paneID, -1)
	if err != nil {
		return err
	}
	if err := sessions.SetActivePane(target.ID); err != nil {
		return err
	}

	sessionName := ""
	if target.Window != nil && target.Window.Session != nil {
		sessionName = target.Window.Session.Name
	}
	a.emitBackendEvent("tmux:pane-focused", map[string]any{
		"sessionName": sessionName,
		"paneId":      target.IDString(),
	})
	return nil
}

// RenamePane updates pane title.
func (a *App) RenamePane(paneID string, title string) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}
	sessionName, err := sessions.RenamePane(paneID, title)
	if err != nil {
		return err
	}
	a.emitBackendEvent("tmux:pane-renamed", map[string]any{
		"sessionName": sessionName,
		"paneId":      paneID,
		"title":       strings.TrimSpace(title),
	})
	return nil
}

// SwapPanes swaps two pane positions in one window.
func (a *App) SwapPanes(sourcePaneID string, targetPaneID string) error {
	sourcePaneID = strings.TrimSpace(sourcePaneID)
	targetPaneID = strings.TrimSpace(targetPaneID)
	if sourcePaneID == "" || targetPaneID == "" {
		return errors.New("both pane ids are required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}
	sessionName, err := sessions.SwapPanes(sourcePaneID, targetPaneID)
	if err != nil {
		return err
	}
	a.emitBackendEvent("tmux:layout-changed", map[string]any{
		"sessionName": sessionName,
	})
	return nil
}

// KillPane closes one pane and updates session state.
func (a *App) KillPane(paneID string) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}
	sessionName, removedSession, err := sessions.KillPane(paneID)
	if err != nil {
		return err
	}
	a.stopOutputBuffer(paneID)
	if removedSession {
		a.emitBackendEvent("tmux:session-destroyed", map[string]string{"name": sessionName})
	} else {
		a.emitBackendEvent("tmux:layout-changed", map[string]any{
			"sessionName": sessionName,
		})
	}
	return nil
}

// ApplyLayoutPreset applies a layout preset to the active window of a session.
// Active-window resolution and preset application are performed atomically inside
// SessionManager to eliminate the TOCTOU gap between reading ActiveWindowID and
// applying the layout. If ActiveWindowID points to a deleted window, the session
// manager falls back to the first available window.
func (a *App) ApplyLayoutPreset(sessionName string, preset string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	preset = strings.TrimSpace(preset)
	if preset == "" {
		return errors.New("preset is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}

	if err := sessions.ApplyLayoutPresetToActiveWindow(sessionName, tmux.LayoutPreset(preset)); err != nil {
		return err
	}
	a.emitBackendEvent("tmux:layout-changed", map[string]any{
		"sessionName": sessionName,
	})
	return nil
}

// GetPaneEnv returns environment variables for one pane on demand.
func (a *App) GetPaneEnv(paneID string) (map[string]string, error) {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return nil, err
	}
	return sessions.GetPaneEnv(paneID)
}
