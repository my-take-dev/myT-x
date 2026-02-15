package main

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

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

// CreateSession creates a new session rooted at path.
// If a session with the same name already exists, a numeric suffix (-2, -3, ...)
// is appended automatically (same deduplication as CreateSessionWithWorktree).
// When enableAgentTeam is true, Agent Teams environment variables are set on the
// session's initial pane so that Claude Code creates team member panes automatically.
func (a *App) CreateSession(rootPath string, sessionName string, enableAgentTeam bool) (tmux.SessionSnapshot, error) {
	sessions, router, err := a.requireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	rootPath = strings.TrimSpace(rootPath)
	sessionName = strings.TrimSpace(sessionName)
	sessionName = a.findAvailableSessionName(sessionName)

	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-c": rootPath,
			"-s": sessionName,
		},
	}
	if enableAgentTeam {
		req.Env = agentTeamEnvVars(sessionName)
	}
	resp := executeRouterRequestFn(router, req)
	if resp.ExitCode != 0 {
		return tmux.SessionSnapshot{}, errors.New(strings.TrimSpace(resp.Stderr))
	}

	createdName := strings.TrimSpace(resp.Stdout)

	// Store git branch metadata for display in the sidebar.
	if gitpkg.IsGitRepository(rootPath) {
		repo, err := gitpkg.Open(rootPath)
		if err != nil {
			slog.Warn("[DEBUG-SESSION] failed to open git repo for session metadata",
				"path", rootPath, "error", err)
		} else {
			branch, brErr := repo.CurrentBranch()
			if brErr != nil {
				slog.Warn("[DEBUG-SESSION] failed to read git branch for session metadata",
					"path", rootPath, "error", brErr)
			} else {
				// wtPath="" signals "no worktree, just tracking git info".
				if setErr := sessions.SetWorktreeInfo(createdName, &tmux.SessionWorktreeInfo{
					RepoPath:   rootPath,
					BranchName: branch,
				}); setErr != nil {
					slog.Warn("[DEBUG-SESSION] failed to store git info for session", "session", createdName, "error", setErr)
				}
			}
		}
	}

	if err := a.storeRootPath(createdName, rootPath); err != nil {
		if rollbackErr := a.rollbackCreatedSession(createdName); rollbackErr != nil {
			a.emitSnapshot()
			return tmux.SessionSnapshot{}, fmt.Errorf("%w (session rollback also failed: %v)", err, rollbackErr)
		}
		a.emitSnapshot()
		return tmux.SessionSnapshot{}, err
	}
	snapshot, activateErr := a.activateCreatedSession(createdName)
	if activateErr != nil {
		return tmux.SessionSnapshot{}, activateErr
	}
	a.emitSnapshot()
	return snapshot, nil
}

// RenameSession renames an existing session.
func (a *App) RenameSession(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	if oldName == "" {
		return errors.New("old session name is required")
	}
	newName = strings.TrimSpace(newName)
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
	if a.getActiveSessionName() == oldName {
		a.setActiveSessionName(newName)
	}
	a.emitSnapshot()
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

	// Capture worktree info before destroying the session.
	worktreeInfo, wtErr := sessions.GetWorktreeInfo(sessionName)

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
	a.stopDetachedOutputBuffers(a.detachStaleOutputBuffers(existingPanes))

	if !deleteWorktree && wtErr != nil {
		// NOTE: Metadata lookup failures are non-fatal when deleteWorktree is false.
		// Session termination must still succeed because cleanup was not requested.
		slog.Warn("[DEBUG-kill] GetWorktreeInfo error", "session", sessionName, "error", wtErr)
	}

	a.emitSnapshot()

	// Worktree cleanup only when the user explicitly chose to delete.
	if deleteWorktree {
		var wtPath string
		var repoPath string
		var branchName string
		if worktreeInfo != nil {
			wtPath = worktreeInfo.Path
			repoPath = worktreeInfo.RepoPath
			branchName = worktreeInfo.BranchName
		}
		a.cleanupSessionWorktree(sessionName, wtPath, repoPath, branchName, wtErr)
	}

	return nil
}

// cleanupSessionWorktree removes a worktree after session destruction.
// Called only when the user explicitly chose to delete the worktree.
func (a *App) cleanupSessionWorktree(sessionName, wtPath, repoPath, branchName string, lookupErr error) {
	if lookupErr != nil {
		slog.Warn("[DEBUG-GIT] failed to resolve worktree metadata before cleanup",
			"session", sessionName, "error", lookupErr)
		a.emitWorktreeCleanupFailure(sessionName, wtPath, lookupErr)
		return
	}
	if wtPath == "" {
		return
	}
	cfg := a.getConfigSnapshot()
	if strings.TrimSpace(repoPath) == "" {
		err := fmt.Errorf("worktree cleanup skipped: repository path is empty for session %s", sessionName)
		slog.Warn("[DEBUG-GIT] failed to clean up worktree", "session", sessionName, "error", err)
		a.emitWorktreeCleanupFailure(sessionName, wtPath, err)
		return
	}

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		slog.Warn("[DEBUG-GIT] failed to open repo for worktree cleanup", "error", err)
		a.emitWorktreeCleanupFailure(sessionName, wtPath, err)
		return
	}

	// Check for uncommitted changes in the worktree.
	// On any error, skip cleanup to avoid data loss (unless ForceCleanup is set).
	if !cfg.Worktree.ForceCleanup && !a.isWorktreeCleanForRemoval(wtPath) {
		err := fmt.Errorf("worktree cleanup skipped due to uncommitted changes or status check failure")
		slog.Warn("[DEBUG-GIT] failed to clean up worktree", "session", sessionName, "path", wtPath, "error", err)
		a.emitWorktreeCleanupFailure(sessionName, wtPath, err)
		return
	}

	if err := repo.RemoveWorktree(wtPath); err != nil {
		if !cfg.Worktree.ForceCleanup {
			slog.Warn("[DEBUG-GIT] failed to remove worktree", "error", err)
			a.emitWorktreeCleanupFailure(sessionName, wtPath, err)
			return
		}
		if fErr := repo.RemoveWorktreeForced(wtPath); fErr != nil {
			slog.Warn("[DEBUG-GIT] failed to force-remove worktree", "error", fErr)
			a.emitWorktreeCleanupFailure(sessionName, wtPath, fErr)
			return
		}
	}

	if err := repo.PruneWorktrees(); err != nil {
		slog.Warn("[DEBUG-GIT] failed to prune worktrees", "error", err)
	}

	a.cleanupOrphanedLocalWorktreeBranch(repo, branchName)
}

func (a *App) cleanupOrphanedLocalWorktreeBranch(repo *gitpkg.Repository, branchName string) {
	branchName = strings.TrimSpace(branchName)
	if repo == nil || branchName == "" {
		return
	}
	deleted, err := repo.CleanupLocalBranchIfOrphaned(branchName)
	if err != nil {
		slog.Warn("[DEBUG-GIT] failed to clean up orphaned local branch",
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
		slog.Warn("[DEBUG-GIT] failed to open worktree for change check, skipping cleanup",
			"path", wtPath, "error", err)
		return false
	}
	hasChanges, chkErr := wtRepo.HasUncommittedChanges()
	if chkErr != nil {
		slog.Warn("[DEBUG-GIT] failed to check uncommitted changes, skipping cleanup",
			"path", wtPath, "error", chkErr)
		return false
	}
	if hasChanges {
		slog.Warn("[DEBUG-GIT] worktree has uncommitted changes, skipping cleanup",
			"path", wtPath)
		return false
	}
	return true
}

// emitWorktreeCleanupFailure notifies the frontend that worktree cleanup failed.
func (a *App) emitWorktreeCleanupFailure(sessionName, wtPath string, err error) {
	if err == nil {
		err = fmt.Errorf("unknown worktree cleanup failure")
	}
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[DEBUG-kill] worktree cleanup failed but app context is nil",
			"session", sessionName, "path", wtPath, "error", err)
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

func (a *App) rollbackCreatedSession(sessionName string) error {
	router, err := a.requireRouter()
	if err != nil {
		return err
	}
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
