package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

// CreateSessionOptions holds the boolean options for session creation APIs.
// This struct replaces consecutive bool parameters (enableAgentTeam, useClaudeEnv,
// usePaneEnv) to eliminate argument-ordering mistakes at call sites.
type CreateSessionOptions struct {
	EnableAgentTeam bool `json:"enable_agent_team"` // set Agent Teams env vars on initial pane
	UseClaudeEnv    bool `json:"use_claude_env"`    // apply claude_env config to panes
	UsePaneEnv      bool `json:"use_pane_env"`      // apply pane_env config to additional panes
}

// agentTeamEnvVars returns the environment variables that signal Claude Code
// to enable Agent Teams mode. The caller is responsible for passing these
// through the IPC request's Env field.
func agentTeamEnvVars(teamName string) map[string]string {
	return map[string]string{
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1",
		"CLAUDE_CODE_TEAM_NAME":                teamName,
		"CLAUDE_CODE_AGENT_ID":                 "lead",
		"CLAUDE_CODE_AGENT_TYPE":               "lead",
	}
}

// QuickStartSession creates a session using the configured default directory
// (config.DefaultSessionDir) or the app launch directory as a fallback.
// If the directory is already in use by an existing session, that session is
// activated and returned instead of creating a new one.
func (a *App) QuickStartSession() (tmux.SessionSnapshot, error) {
	cfg := a.getConfigSnapshot()
	dir := strings.TrimSpace(cfg.DefaultSessionDir)
	if dir == "" {
		dir = a.launchDir
	}
	if dir == "" {
		return tmux.SessionSnapshot{}, errors.New("no directory available for quick start session")
	}

	// [C2] Environment variables and ~ are expanded by config.validateDefaultSessionDir
	// at load/save time. This guard handles direct API calls with unexpanded paths.
	if strings.HasPrefix(dir, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[1:])
		}
	}

	// Resolve symlinks up front to avoid creating duplicate sessions for
	// path aliases that point to the same directory.
	if resolvedDir, evalErr := filepath.EvalSymlinks(dir); evalErr == nil {
		dir = resolvedDir
	} else if !errors.Is(evalErr, os.ErrNotExist) {
		slog.Debug("[DEBUG-SESSION] quick start path canonicalization skipped",
			"path", dir, "error", evalErr)
	}

	// [I1] os.MkdirAll is idempotent; calling it on an existing directory returns nil.
	// We call it unconditionally to avoid the TOCTOU race of Stat → MkdirAll.
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("quick start directory create failed: %w", mkErr)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("quick start directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return tmux.SessionSnapshot{}, fmt.Errorf("quick start path is not a directory: %s", dir)
	}

	// If directory is already used by an existing session, activate it.
	// NOTE: This check is intentionally best-effort. If the session disappears
	// between findSessionByRootPath and the snapshot loop (TOCTOU window), we
	// safely fall through to CreateSession.
	if conflict := a.findSessionByRootPath(dir); conflict != "" {
		// NOTE: findSessionByRootPath internally calls requireSessions().Snapshot()
		// but only returns the session name. We need a fresh SessionSnapshot to
		// return to the caller, hence this second requireSessions() call.
		// The Snapshot() call itself is a lightweight in-memory copy.
		sessions, sErr := a.requireSessions()
		if sErr == nil {
			for _, s := range sessions.Snapshot() {
				if s.Name == conflict {
					a.setActiveSessionName(conflict)
					a.requestSnapshot(true)
					a.emitBackendEvent("tmux:active-session", map[string]any{"name": conflict})
					return s, nil
				}
			}
		} else {
			slog.Debug("[DEBUG-SESSION] quick start conflict snapshot unavailable; creating new session",
				"session", conflict, "path", dir, "error", sErr)
		}
		// NOTE: If we reach here, the conflict session disappeared between
		// findSessionByRootPath and the snapshot loop (TOCTOU window).
		// Fall through to CreateSession as documented above.
	}

	sessionName := filepath.Base(dir)
	if sessionName == "" || sessionName == "." || sessionName == string(os.PathSeparator) || (len(sessionName) == 2 && sessionName[1] == ':') {
		sessionName = "quick-session"
	}
	sessionName = sanitizeSessionName(sessionName, "quick-session")

	return a.CreateSession(dir, sessionName, CreateSessionOptions{})
}

// sanitizeSessionNameRegex collapses runs of characters that are invalid in
// tmux session names (dots, colons) into a single hyphen.
// Compiled at package level to avoid re-compilation per call.
var sanitizeSessionNameRegex = regexp.MustCompile(`[.:]+`)

// consecutiveHyphenRegex collapses runs of multiple hyphens into one.
var consecutiveHyphenRegex = regexp.MustCompile(`-{2,}`)

// sanitizeSessionName replaces characters that are invalid in tmux session
// names with hyphens. tmux rejects names containing '.' and ':'.
// fallback is returned when the sanitized name is empty.
func sanitizeSessionName(name, fallback string) string {
	sanitized := sanitizeSessionNameRegex.ReplaceAllString(name, "-")
	sanitized = consecutiveHyphenRegex.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return fallback
	}
	return sanitized
}

// CreateSession creates a new session rooted at path.
// If a session with the same name already exists, a numeric suffix (-2, -3, ...)
// is appended automatically (same deduplication as CreateSessionWithWorktree).
// When opts.EnableAgentTeam is true, Agent Teams environment variables are set on the
// session's initial pane so that Claude Code creates team member panes automatically.
func (a *App) CreateSession(rootPath string, sessionName string, opts CreateSessionOptions) (snapshot tmux.SessionSnapshot, retErr error) {
	sessions, router, err := a.requireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return tmux.SessionSnapshot{}, errors.New("root path is required")
	}
	sessionName = strings.TrimSpace(sessionName)
	sessionName = sanitizeSessionName(sessionName, "session")
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("session name is required")
	}
	sessionName = a.findAvailableSessionName(sessionName)
	createdName := ""
	sessionMayExist := false
	defer func() {
		if !sessionMayExist {
			return
		}
		if retErr == nil {
			// Emit a snapshot for successful create flows so sidebar/session-list
			// subscribers receive the latest state.
			a.requestSnapshot(true)
			return
		}
		if createdName != "" {
			slog.Debug("[DEBUG-SESSION] rolling back session after create failure",
				"session", createdName, "error", retErr)
			if rollbackErr := a.rollbackCreatedSession(createdName); rollbackErr != nil {
				retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackErr)
			}
		}
		// Emit snapshot on rollback/error paths so UI state is reconciled with
		// the latest backend state.
		a.requestSnapshot(true)
	}()

	createdName, err = a.createSessionForDirectory(router, rootPath, sessionName, opts)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	sessionMayExist = true

	// Set session-level env flags before any additional pane can be created.
	applySessionEnvFlags(sessions, createdName, opts.UseClaudeEnv, opts.UsePaneEnv)

	// Store git branch metadata for display in the sidebar.
	// NOTE: This enrichment is best-effort. Session creation must continue even if
	// git probing fails because tmux session creation already succeeded.
	enrichSessionGitMetadata(sessions, createdName, rootPath)

	if err := a.storeRootPath(createdName, rootPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}
	snapshot, retErr = a.activateCreatedSession(createdName)
	return snapshot, retErr
}

// RenameSession renames an existing session.
func (a *App) RenameSession(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	if oldName == "" {
		return errors.New("old session name is required")
	}
	newName = strings.TrimSpace(newName)
	newName = sanitizeSessionName(newName, "session")
	if newName == "" {
		return errors.New("new session name is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}
	if err := sessions.RenameSession(oldName, newName); err != nil {
		return err
	}
	if a.mcpManager != nil {
		a.mcpManager.CleanupSession(oldName)
	}
	if a.getActiveSessionName() == oldName {
		a.setActiveSessionName(newName)
	}
	// NOTE: RenameSession does not call requestSnapshot(true) directly.
	// Instead, the snapshot is triggered automatically by emitBackendEvent
	// via snapshotEventPolicies["tmux:session-renamed"] = {trigger: true, bypassDebounce: true}.
	// The "tmux:session-renamed" event provides immediate UI feedback (e.g. sidebar rename animation)
	// while the triggered snapshot reconciles the full state.
	a.emitBackendEvent("tmux:session-renamed", map[string]any{
		"oldName": oldName,
		"newName": newName,
	})
	return nil
}

// KillSession closes one session.
// If deleteWorktree is true and the session has an associated worktree,
// the worktree is removed after the session is destroyed.
// The decision to delete is made by the user via the KillSessionDialog.
func (a *App) KillSession(sessionName string, deleteWorktree bool) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	sessions, router, err := a.requireSessionsAndRouter()
	if err != nil {
		return err
	}

	// Capture worktree metadata before destroying the session so it remains
	// available for cleanup and diagnostics even after kill-session succeeds.
	worktreeInfo, wtErr := sessions.GetWorktreeInfo(sessionName)
	if !deleteWorktree && wtErr != nil {
		slog.Debug("[DEBUG-GIT] failed to resolve worktree metadata for killed session (deleteWorktree=false)",
			"session", sessionName, "error", wtErr)
	}

	resp := executeRouterRequestFn(router, ipc.TmuxRequest{
		Command: "kill-session",
		Flags: map[string]any{
			"-t": sessionName,
		},
	})
	if resp.ExitCode != 0 {
		return errors.New(strings.TrimSpace(resp.Stderr))
	}

	// Stop pane buffers that no longer exist (lightweight pane ID lookup).
	existingPanes := sessions.ActivePaneIDs()
	a.cleanupDetachedPaneStates(a.detachStaleOutputBuffers(existingPanes))

	if a.getActiveSessionName() == sessionName {
		a.setActiveSessionName("")
	}
	// Release MCP instance state for the destroyed session.
	if a.mcpManager != nil {
		a.mcpManager.CleanupSession(sessionName)
	}
	a.requestSnapshot(true)

	// Worktree cleanup only when the user explicitly chose to delete.
	if deleteWorktree {
		if wtErr != nil {
			slog.Warn("[WARN-GIT] failed to resolve worktree metadata before cleanup",
				"session", sessionName, "error", wtErr)
			a.emitWorktreeCleanupFailure(sessionName, "", wtErr)
			// NOTE: Session termination already succeeded. Treat metadata lookup
			// failure as a non-fatal cleanup warning to avoid retry loops that
			// can surface "session not found" after successful kill-session.
			return nil
		} else if worktreeInfo != nil {
			a.cleanupSessionWorktree(worktreeCleanupParams{
				SessionName: sessionName,
				WtPath:      worktreeInfo.Path,
				RepoPath:    worktreeInfo.RepoPath,
				BranchName:  worktreeInfo.BranchName,
			})
		} else {
			slog.Debug("[DEBUG-SESSION] deleteWorktree requested but session has no worktree metadata",
				"session", sessionName)
		}
	}

	return nil
}

// worktreeCleanupParams holds parameters for cleanupSessionWorktree.
type worktreeCleanupParams struct {
	SessionName string
	WtPath      string
	RepoPath    string
	BranchName  string
}

// cleanupSessionWorktree removes a worktree after session destruction.
// Called only when the user explicitly chose to delete the worktree.
func (a *App) cleanupSessionWorktree(params worktreeCleanupParams) {
	wtPath := strings.TrimSpace(params.WtPath)
	if wtPath == "" {
		slog.Debug("[DEBUG-GIT] skip worktree cleanup: worktree path is empty",
			"session", params.SessionName)
		return
	}
	cfg := a.getConfigSnapshot()
	repoPath := strings.TrimSpace(params.RepoPath)
	if repoPath == "" {
		err := fmt.Errorf("worktree cleanup skipped: repository path is empty for session %s", params.SessionName)
		slog.Warn("[WARN-GIT] failed to clean up worktree", "session", params.SessionName, "error", err)
		a.emitWorktreeCleanupFailure(params.SessionName, wtPath, err)
		return
	}

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		slog.Warn("[WARN-GIT] failed to open repo for worktree cleanup", "error", err)
		a.emitWorktreeCleanupFailure(params.SessionName, wtPath, err)
		return
	}

	// Check for uncommitted changes in the worktree.
	// On any error, skip cleanup to avoid data loss (unless ForceCleanup is set).
	if !cfg.Worktree.ForceCleanup && !a.isWorktreeCleanForRemoval(wtPath) {
		err := fmt.Errorf("worktree cleanup skipped due to uncommitted changes or status check failure")
		slog.Warn("[WARN-GIT] failed to clean up worktree", "session", params.SessionName, "path", wtPath, "error", err)
		a.emitWorktreeCleanupFailure(params.SessionName, wtPath, err)
		return
	}

	if err := repo.RemoveWorktree(wtPath); err != nil {
		if !cfg.Worktree.ForceCleanup {
			slog.Warn("[WARN-GIT] failed to remove worktree", "error", err)
			a.emitWorktreeCleanupFailure(params.SessionName, wtPath, err)
			return
		}
		if fErr := repo.RemoveWorktreeForced(wtPath); fErr != nil {
			slog.Warn("[WARN-GIT] failed to force-remove worktree", "error", fErr)
			a.emitWorktreeCleanupFailure(params.SessionName, wtPath, fErr)
			return
		}
	}

	if err := repo.PruneWorktrees(); err != nil {
		slog.Warn("[WARN-GIT] failed to prune worktrees", "error", err)
	}

	a.cleanupOrphanedLocalWorktreeBranch(repo, params.BranchName)
}

func (a *App) cleanupOrphanedLocalWorktreeBranch(repo *gitpkg.Repository, branchName string) {
	branchName = strings.TrimSpace(branchName)
	if repo == nil {
		slog.Debug("[DEBUG-GIT] skip orphaned branch cleanup: repository is nil", "branch", branchName)
		return
	}
	if branchName == "" {
		slog.Debug("[DEBUG-GIT] skip orphaned branch cleanup: branch name is empty")
		return
	}
	deleted, err := repo.CleanupLocalBranchIfOrphaned(branchName)
	if err != nil {
		slog.Warn("[WARN-GIT] failed to clean up orphaned local branch",
			"branch", branchName, "error", err)
		return
	}
	if deleted {
		slog.Debug("[DEBUG-GIT] removed orphaned local branch created for worktree",
			"branch", branchName)
	}
}

// isWorktreeCleanForRemoval returns true if the worktree has no uncommitted changes.
// On any error (open / check), it returns false to prevent data loss.
func (a *App) isWorktreeCleanForRemoval(wtPath string) bool {
	wtRepo, err := gitpkg.Open(wtPath)
	if err != nil {
		slog.Warn("[WARN-GIT] failed to open worktree for change check, skipping cleanup",
			"path", wtPath, "error", err)
		return false
	}
	hasChanges, chkErr := wtRepo.HasUncommittedChanges()
	if chkErr != nil {
		slog.Warn("[WARN-GIT] failed to check uncommitted changes, skipping cleanup",
			"path", wtPath, "error", chkErr)
		return false
	}
	if hasChanges {
		slog.Warn("[WARN-GIT] worktree has uncommitted changes, skipping cleanup",
			"path", wtPath)
		return false
	}
	return true
}

// emitWorktreeCleanupFailure notifies the frontend that worktree cleanup failed.
func (a *App) emitWorktreeCleanupFailure(sessionName, wtPath string, err error) {
	// NOTE: err==nil is defensively handled here to ensure the event payload always
	// contains a non-empty error string. All current callers pass a non-nil err, but
	// this guard prevents silent data loss if a future call site omits the error.
	if err == nil {
		err = fmt.Errorf("unknown worktree cleanup failure")
	}
	ctx := a.runtimeContext()
	if ctx == nil {
		// NOTE: During shutdown, runtimeContext() may return nil. Event is intentionally
		// dropped as no frontend receiver exists. The error is still logged in the
		// slog.Warn below for post-mortem diagnostics.
		slog.Warn("[WARN-WORKTREE] emitWorktreeCleanupFailure: runtime context is nil, event dropped",
			"sessionName", sessionName, "path", wtPath, "error", err)
		return
	}
	a.emitRuntimeEventWithContext(ctx, "worktree:cleanup-failed", map[string]any{
		"sessionName": sessionName,
		"path":        wtPath,
		"error":       err.Error(),
	})
}

// GetSessionEnv returns environment variables for one session on demand.
func (a *App) GetSessionEnv(sessionName string) (map[string]string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, err
	}
	return sessions.GetSessionEnv(sessionName)
}

// applySessionEnvFlags sets session-level UseClaudeEnv and UsePaneEnv flags
// on the SessionManager. These flags control whether additional panes inherit
// claude_env / pane_env variables via buildPaneEnvForSession.
//
// IMPORTANT: Every session creation path (CreateSession, CreateSessionWithWorktree,
// CreateSessionWithExistingWorktree) must call this function after
// createSessionForDirectory succeeds. When adding a new session creation path,
// ensure applySessionEnvFlags is called to avoid silent flag-loss.
//
// NOTE: Failure is non-fatal (Warn only). The only realistic failure case is
// session-name mismatch in SessionManager, which cannot happen here because
// the session was just created successfully by createSessionForDirectory.
// Aborting the entire session creation for a flag-storage failure would be
// disproportionate; the session remains fully functional without these flags
// (additional panes simply won't inherit the configured env).
func applySessionEnvFlags(sm *tmux.SessionManager, sessionName string, useClaudeEnv, usePaneEnv bool) {
	if useClaudeEnv {
		if setErr := sm.SetUseClaudeEnv(sessionName, useClaudeEnv); setErr != nil {
			slog.Warn("[WARN-ENV] failed to set UseClaudeEnv flag", "session", sessionName, "error", setErr)
		}
	}
	if usePaneEnv {
		if setErr := sm.SetUsePaneEnv(sessionName, usePaneEnv); setErr != nil {
			slog.Warn("[WARN-ENV] failed to set UsePaneEnv flag", "session", sessionName, "error", setErr)
		}
	}
}

// enrichSessionGitMetadata probes the rootPath for git information and stores
// branch metadata on the session for sidebar display. This is best-effort:
// any failure is logged but does not abort session creation.
func enrichSessionGitMetadata(sessions *tmux.SessionManager, sessionName, rootPath string) {
	if !gitpkg.IsGitRepository(rootPath) {
		return
	}
	repo, err := gitpkg.Open(rootPath)
	if err != nil {
		slog.Warn("[WARN-SESSION] failed to open git repo for session metadata",
			"path", rootPath, "error", err)
		return
	}
	branch, err := repo.CurrentBranch()
	if err != nil {
		slog.Warn("[WARN-SESSION] failed to read git branch for session metadata",
			"path", rootPath, "error", err)
		return
	}
	// wtPath="" signals "no worktree, just tracking git info".
	if setErr := sessions.SetWorktreeInfo(sessionName, &tmux.SessionWorktreeInfo{
		RepoPath:   rootPath,
		BranchName: branch,
	}); setErr != nil {
		slog.Warn("[WARN-SESSION] failed to store git info for session",
			"session", sessionName, "error", setErr)
	}
}

func (a *App) rollbackCreatedSession(sessionName string) error {
	router, err := a.requireRouter()
	if err != nil {
		return err
	}
	return rollbackSessionByRouter(router, sessionName)
}

func rollbackSessionByRouter(router *tmux.CommandRouter, sessionName string) error {
	resp := executeRouterRequestFn(router, ipc.TmuxRequest{
		Command: "kill-session",
		Flags: map[string]any{
			"-t": sessionName,
		},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("failed to rollback session: %s", strings.TrimSpace(resp.Stderr))
	}
	return nil
}

// storeRootPath saves the root directory for a newly created session.
func (a *App) storeRootPath(sessionName, rootPath string) error {
	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}
	if setErr := sessions.SetRootPath(sessionName, rootPath); setErr != nil {
		return fmt.Errorf("failed to set root path for conflict detection: %w", setErr)
	}
	return nil
}

// activateCreatedSession finds the just-created session in the snapshot list,
// sets it as the active session, and returns its snapshot.
func (a *App) activateCreatedSession(createdName string) (tmux.SessionSnapshot, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	snapshots := sessions.Snapshot()
	for _, snapshot := range snapshots {
		if snapshot.Name == createdName {
			a.setActiveSessionName(snapshot.Name)
			return snapshot, nil
		}
	}
	return tmux.SessionSnapshot{}, fmt.Errorf("created session not found: %s", createdName)
}

// pathsEqualFold compares two file paths case-insensitively after normalization.
// Suitable for Windows path comparison where case is insignificant.
func pathsEqualFold(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// findSessionByRootPath returns the session name that uses the given
// root path, or "" if no active session uses it.
func (a *App) findSessionByRootPath(dir string) string {
	sessions, err := a.requireSessions()
	if err != nil {
		// During startup/shutdown, session manager access can fail transiently.
		// Treat this as "no conflict" to avoid blocking session-creation flows.
		slog.Debug("[DEBUG-SESSION] findSessionByRootPath fallback to no-conflict",
			"path", dir, "error", err)
		return ""
	}
	normalizedPath := filepath.Clean(dir)
	snapshots := sessions.Snapshot()
	for _, s := range snapshots {
		// Worktreeセッションの実際の作業ディレクトリはWorktreePathであり、
		// RootPath(リポジトリパス)ではない。他セッションをブロックしない。
		if s.Worktree != nil && s.Worktree.Path != "" {
			continue
		}
		if s.RootPath != "" && pathsEqualFold(s.RootPath, normalizedPath) {
			return s.Name
		}
	}
	return ""
}

// CheckDirectoryConflict checks whether the given directory is already
// used as the root path by an active session.
// Returns the session name if conflict exists, or "".
func (a *App) CheckDirectoryConflict(dir string) string {
	return a.findSessionByRootPath(strings.TrimSpace(dir))
}
