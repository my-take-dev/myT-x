package main

import (
	"errors"
	"log/slog"
	"strings"

	"myT-x/internal/install"
	"myT-x/internal/session"
	"myT-x/internal/tmux"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// CreateSessionOptions holds the boolean options for session creation APIs.
// This struct replaces consecutive bool parameters (enableAgentTeam, useClaudeEnv,
// usePaneEnv) to eliminate argument-ordering mistakes at call sites.
type CreateSessionOptions struct {
	EnableAgentTeam     bool `json:"enable_agent_team"`      // set Agent Teams env vars on initial pane
	UseClaudeEnv        bool `json:"use_claude_env"`         // apply claude_env config to panes
	UsePaneEnv          bool `json:"use_pane_env"`           // apply pane_env config to additional panes
	UseSessionPaneScope bool `json:"use_session_pane_scope"` // set MYTX_SESSION on panes + scope list-panes
}

// toSessionOpts maps the Wails-bound CreateSessionOptions to the session
// package's internal options type.
func (o CreateSessionOptions) toSessionOpts() session.CreateSessionOptions {
	return session.CreateSessionOptions{
		EnableAgentTeam:     o.EnableAgentTeam,
		UseClaudeEnv:        o.UseClaudeEnv,
		UsePaneEnv:          o.UsePaneEnv,
		UseSessionPaneScope: o.UseSessionPaneScope,
	}
}

// ---------------------------------------------------------------------------
// Wails-bound thin wrappers — delegate to sessionService
// ---------------------------------------------------------------------------

// QuickStartSession creates a session using the configured default directory
// (config.DefaultSessionDir) or the app launch directory as a fallback.
// If the directory is already in use by an existing session, that session is
// activated and returned instead of creating a new one.
// Wails-bound: called from the frontend.
func (a *App) QuickStartSession() (tmux.SessionSnapshot, error) {
	return a.sessionService.QuickStartSession(a.launchDir)
}

// CreateSession creates a new session rooted at path.
// If a session with the same name already exists, a numeric suffix (-2, -3, ...)
// is appended automatically (same deduplication as CreateSessionWithWorktree).
// When opts.EnableAgentTeam is true, Agent Teams environment variables are set on the
// session's initial pane so that Claude Code creates team member panes automatically.
// Wails-bound: called from the frontend.
func (a *App) CreateSession(rootPath string, sessionName string, opts CreateSessionOptions) (tmux.SessionSnapshot, error) {
	return a.sessionService.CreateSession(rootPath, sessionName, opts.toSessionOpts())
}

// RenameSession renames an existing session.
// Wails-bound: called from the frontend.
func (a *App) RenameSession(oldName, newName string) error {
	return a.sessionService.RenameSession(oldName, newName)
}

// KillSession closes one session.
// If deleteWorktree is true and the session has an associated worktree,
// the worktree is removed after the session is destroyed.
// The decision to delete is made by the user via the KillSessionDialog.
// Wails-bound: called from the frontend.
func (a *App) KillSession(sessionName string, deleteWorktree bool) error {
	return a.sessionService.KillSession(sessionName, deleteWorktree)
}

// GetSessionEnv returns environment variables for one session on demand.
// Wails-bound: called from the frontend.
func (a *App) GetSessionEnv(sessionName string) (map[string]string, error) {
	return a.sessionService.GetSessionEnv(sessionName)
}

// ListSessions returns current session snapshots.
// Wails-bound: called from the frontend.
func (a *App) ListSessions() []tmux.SessionSnapshot {
	return a.sessionService.ListSessions()
}

// SetActiveSession sets current active session for status line and UI.
// Wails-bound: called from the frontend.
func (a *App) SetActiveSession(sessionName string) {
	a.sessionService.SetActive(sessionName)
}

// GetActiveSession returns active session name.
// Wails-bound: called from the frontend.
func (a *App) GetActiveSession() string {
	return a.sessionService.GetActiveSessionName()
}

// CheckDirectoryConflict checks whether the given directory is already
// used as the root path by an active session.
// Returns the session name if conflict exists, or "".
// Wails-bound: called from the frontend.
func (a *App) CheckDirectoryConflict(dir string) string {
	return a.sessionService.CheckDirectoryConflict(dir)
}

// ---------------------------------------------------------------------------
// App-owned methods — not delegated to sessionService
// ---------------------------------------------------------------------------

// OpenDirectoryInExplorer opens the session's working directory in Windows Explorer.
// If the session has a worktree, Worktree.Path is used; otherwise RootPath is used.
// Wails-bound: called from the frontend.
func (a *App) OpenDirectoryInExplorer(sessionName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	found, err := a.sessionService.FindSessionSnapshotByName(sessionName)
	if err != nil {
		return err
	}

	targetPath := session.ResolveSessionDirectory(found)
	if targetPath == "" {
		return errors.New("no directory associated with session: " + sessionName)
	}

	// Copy locals for goroutine closure to prevent future data-race risk
	// if this function is later modified to reassign these variables.
	pathCopy := targetPath
	nameCopy := sessionName
	go func() {
		if err := a.openExplorerFn(pathCopy); err != nil {
			slog.Warn("[WARN-EXPLORER] failed to open explorer",
				"path", pathCopy, "session", nameCopy, "error", err)
		}
	}()
	return nil
}

// PickSessionDirectory opens a directory picker for new session root.
// Wails-bound: called from the frontend.
func (a *App) PickSessionDirectory() (string, error) {
	ctx := a.runtimeContext()
	if ctx == nil {
		return "", errors.New("app context is not ready")
	}
	dir, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Select Session Root Directory",
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(dir), nil
}

// DetachSession currently keeps process alive and only emits UI event.
// Wails-bound: called from the frontend.
func (a *App) DetachSession(sessionName string) {
	a.emitRuntimeEvent("tmux:session-detached", map[string]string{"name": sessionName})
}

// IsAgentTeamsAvailable reports whether Agent Teams can be started.
// Returns true when tmux CLI shim is installed and available on PATH.
// Wails-bound: called from the frontend.
func (a *App) IsAgentTeamsAvailable() bool {
	router, err := a.requireRouter()
	if err != nil {
		return false
	}
	return router.ShimAvailable()
}

// InstallTmuxShim triggers shim installer manually.
// Wails-bound: called from the frontend.
func (a *App) InstallTmuxShim() (install.ShimInstallResult, error) {
	result, err := ensureShimInstalledFn(a.workspace)
	if err != nil {
		return install.ShimInstallResult{}, err
	}
	if router, guardErr := a.requireRouter(); guardErr == nil {
		router.SetShimAvailable(true)
	}
	a.emitRuntimeEvent("tmux:shim-installed", result)
	return result, nil
}
