package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/procutil"
	"myT-x/internal/tmux"
)

var executeRouterRequestFn = func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
	return router.Execute(req)
}

var currentBranchFn = func(repo *gitpkg.Repository) (string, error) {
	return repo.CurrentBranch()
}

var executeSetupCommandFn = func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, shell, shellFlag, script)
	cmd.Dir = dir
	procutil.HideWindow(cmd)
	return cmd.CombinedOutput()
}

// WorktreeSessionOptions holds options for creating a session with a worktree.
//
// Mode semantics (invariant):
//   - BranchName is required (non-empty) → Named branch mode
//   - BaseBranch == ""  → Uses current HEAD as the base for the worktree
//
// NOTE: Detached HEAD mode was removed from the UI. Existing detached worktrees
// (via CreateSessionWithExistingWorktree) are still supported and can be
// promoted via PromoteWorktreeToBranch.
type WorktreeSessionOptions struct {
	BranchName       string `json:"branch_name"`        // required: branch name for the new worktree
	BaseBranch       string `json:"base_branch"`        // empty = current HEAD
	PullBeforeCreate bool   `json:"pull_before_create"` // pull latest before creating worktree
	EnableAgentTeam  bool   `json:"enable_agent_team"`  // set Agent Teams env vars on initial pane
}

// WorktreeStatus holds the pre-close status of a worktree session.
type WorktreeStatus struct {
	HasWorktree    bool   `json:"has_worktree"`
	HasUncommitted bool   `json:"has_uncommitted"`
	HasUnpushed    bool   `json:"has_unpushed"`
	BranchName     string `json:"branch_name"`
	IsDetached     bool   `json:"is_detached"`
}

// CreateSessionWithWorktree creates a new session backed by a git worktree.
// The worktree is placed at {parentDir}/{repoName}.wt/{identifier}.
func (a *App) CreateSessionWithWorktree(
	repoPath string,
	sessionName string,
	opts WorktreeSessionOptions,
) (snapshot tmux.SessionSnapshot, retErr error) {
	sessions, router, err := a.requireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	repoPath = strings.TrimSpace(repoPath)
	sessionName = strings.TrimSpace(sessionName)
	cfg := a.getConfigSnapshot()
	createdName := ""
	wtPath := ""
	worktreeCreated := false
	var repo *gitpkg.Repository
	defer func() {
		if retErr == nil {
			return
		}
		if createdName != "" {
			if rollbackSessionErr := a.rollbackCreatedSession(createdName); rollbackSessionErr != nil {
				retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackSessionErr)
			}
			// Notify frontend after session rollback for UI consistency (#69).
			a.emitSnapshot()
		}
		if worktreeCreated && repo != nil && wtPath != "" {
			if rollbackErr := rollbackWorktree(repo, wtPath); rollbackErr != nil {
				retErr = fmt.Errorf("%w (worktree rollback also failed: %v)", retErr, rollbackErr)
			}
		}
	}()

	if !cfg.Worktree.Enabled {
		return tmux.SessionSnapshot{}, fmt.Errorf("worktree feature is disabled in config")
	}

	if !gitpkg.IsGitRepository(repoPath) {
		return tmux.SessionSnapshot{}, fmt.Errorf("not a git repository: %s", repoPath)
	}

	if opts.BranchName == "" {
		return tmux.SessionSnapshot{}, fmt.Errorf("branch name is required for new worktree creation")
	}

	// Deduplicate session name BEFORE creating the worktree to avoid
	// orphaned branches when session creation fails due to name collision.
	sessionName = a.findAvailableSessionName(sessionName)

	repo, err = gitpkg.Open(repoPath)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to open repository: %w", err)
	}

	isDetached := false
	resolvedBaseBranch := ""
	wtPath, isDetached, resolvedBaseBranch, err = a.createWorktreeForSession(repo, repoPath, sessionName, opts)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	worktreeCreated = true

	createdName, err = a.createSessionForDirectory(router, wtPath, sessionName, opts.EnableAgentTeam)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Store worktree metadata on the session.
	branchName := opts.BranchName
	if isDetached {
		branchName = ""
	}
	if err := sessions.SetWorktreeInfo(createdName, &tmux.SessionWorktreeInfo{
		Path:       wtPath,
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseBranch: resolvedBaseBranch,
		IsDetached: isDetached,
	}); err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to set worktree info: %w", err)
	}

	if err := a.storeRootPath(createdName, repoPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Copy configured files (e.g. .env) from repo to worktree.
	if copyFailures := copyConfigFilesToWorktree(repoPath, wtPath, cfg.Worktree.CopyFiles); len(copyFailures) > 0 {
		a.emitRuntimeEvent("worktree:copy-files-failed", map[string]any{
			"sessionName": createdName,
			"files":       copyFailures,
		})
	}

	// NOTE: Setup scripts run regardless of copy failures because they are
	// independent operations. Copy files (e.g. .env) are best-effort;
	// blocking setup scripts on copy failure would degrade the user experience
	// for unrelated issues.

	// Run setup scripts asynchronously if configured.
	if len(cfg.Worktree.SetupScripts) > 0 {
		a.setupWG.Add(1)
		go func() {
			defer a.setupWG.Done()
			defer func() {
				recoverBackgroundPanic("worktree-setup-scripts", recover())
			}()
			a.runSetupScripts(wtPath, createdName, cfg.Shell, cfg.Worktree.SetupScripts)
		}()
	}

	snapshot, retErr = a.activateCreatedSession(createdName)
	return snapshot, retErr
}

func (a *App) createSessionForDirectory(
	router *tmux.CommandRouter,
	sessionDir,
	sessionName string,
	enableAgentTeam bool,
) (string, error) {
	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-c": sessionDir,
			"-s": sessionName,
		},
	}
	if enableAgentTeam {
		req.Env = agentTeamEnvVars(sessionName)
	}
	resp := executeRouterRequestFn(router, req)
	if resp.ExitCode != 0 {
		return "", fmt.Errorf("failed to create session: %s", strings.TrimSpace(resp.Stderr))
	}
	return strings.TrimSpace(resp.Stdout), nil
}

// createWorktreeForSession creates the git worktree for a new session.
// Handles pull, path generation, validation, and the actual worktree creation.
func (a *App) createWorktreeForSession(
	repo *gitpkg.Repository, repoPath, sessionName string, opts WorktreeSessionOptions,
) (wtPath string, isDetached bool, resolvedBaseBranch string, err error) {
	if opts.PullBeforeCreate {
		if pullErr := repo.Pull(); pullErr != nil {
			slog.Warn("[DEBUG-GIT] pull before worktree creation failed",
				"error", pullErr, "repoPath", repoPath)
			return "", false, "", fmt.Errorf("failed to pull latest changes: %w", pullErr)
		}
	}

	isDetached = opts.BranchName == ""

	var identifier string
	if isDetached {
		identifier = gitpkg.SanitizeCustomName(sessionName)
		if identifier == "" || identifier == "work" {
			identifier = fmt.Sprintf("wt-%d", time.Now().UnixMilli())
		}
	} else {
		identifier = gitpkg.SanitizeCustomName(opts.BranchName)
	}

	wtPath = gitpkg.FindAvailableWorktreePath(gitpkg.GenerateWorktreePath(repoPath, identifier))

	if err := gitpkg.ValidateWorktreePath(wtPath); err != nil {
		return "", false, "", fmt.Errorf("invalid worktree path: %w", err)
	}

	wtDir := gitpkg.GenerateWorktreeDirPath(repoPath)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", false, "", fmt.Errorf("failed to create worktree directory %s: %w", wtDir, err)
	}

	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		// Resolve actual branch name for display purposes.
		if resolved, brErr := repo.CurrentBranch(); brErr == nil && resolved != "" {
			baseBranch = resolved
		} else {
			baseBranch = "HEAD"
		}
	}

	if isDetached {
		if err := repo.CreateWorktreeDetached(wtPath, baseBranch); err != nil {
			return "", false, "", fmt.Errorf("failed to create detached worktree: %w", err)
		}
	} else {
		if err := repo.CreateWorktree(wtPath, opts.BranchName, baseBranch); err != nil {
			return "", false, "", fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	slog.Debug("[DEBUG-GIT] worktree created",
		"path", wtPath, "repoPath", repoPath, "detached", isDetached)

	return wtPath, isDetached, baseBranch, nil
}

// rollbackWorktree removes a worktree and prunes orphaned entries.
// Returns the removal error (if any) for inclusion in the caller's error message.
func rollbackWorktree(repo *gitpkg.Repository, wtPath string) error {
	var rollbackErr error
	if rmErr := repo.RemoveWorktreeForced(wtPath); rmErr != nil {
		slog.Warn("[DEBUG-GIT] failed to rollback worktree", "error", rmErr)
		rollbackErr = rmErr
	}
	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[DEBUG-GIT] failed to prune worktrees during rollback", "error", pruneErr)
	}
	return rollbackErr
}

// copyConfigFilesToWorktree copies configured files (e.g. .env) from the
// repository root to the worktree. Returns a list of files that failed to copy.
// Missing source files are silently skipped (common for optional files like .env).
func copyConfigFilesToWorktree(repoPath, wtPath string, files []string) []string {
	var failures []string
	for _, file := range files {
		cleaned := filepath.Clean(file)
		if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
			slog.Warn("[DEBUG-GIT] skipping unsafe copy_files entry", "file", file)
			continue
		}
		// SECURITY: Verify the resolved path is still within the target directories.
		// Clean base paths to normalize trailing separators, then append
		// separator to prevent prefix collisions (e.g. /repo matching /repo-evil).
		src := filepath.Join(repoPath, cleaned)
		dst := filepath.Join(wtPath, cleaned)
		repoBase := filepath.Clean(repoPath) + string(filepath.Separator)
		wtBase := filepath.Clean(wtPath) + string(filepath.Separator)
		if !strings.HasPrefix(src, repoBase) || !strings.HasPrefix(dst, wtBase) {
			slog.Warn("[DEBUG-GIT] skipping copy_files entry escaping base directory", "file", file)
			continue
		}
		data, readErr := os.ReadFile(src)
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				slog.Warn("[DEBUG-GIT] failed to read source file for copy",
					"src", src, "error", readErr)
				failures = append(failures, file)
			}
			continue
		}
		if dstDir := filepath.Dir(dst); dstDir != "." {
			if mkErr := os.MkdirAll(dstDir, 0o755); mkErr != nil {
				slog.Warn("[DEBUG-GIT] failed to create destination directory",
					"dir", dstDir, "error", mkErr)
				failures = append(failures, file)
				continue
			}
		}
		if writeErr := os.WriteFile(dst, data, 0o600); writeErr != nil {
			slog.Warn("[DEBUG-GIT] failed to copy file to worktree",
				"src", src, "dst", dst, "error", writeErr)
			failures = append(failures, file)
		}
	}
	return failures
}

// IsGitRepository checks if the given path is a git repository.
func (a *App) IsGitRepository(path string) bool {
	return gitpkg.IsGitRepository(strings.TrimSpace(path))
}

// ListBranches returns all local branch names for the repository at the given path.
func (a *App) ListBranches(repoPath string) ([]string, error) {
	repo, err := gitpkg.Open(strings.TrimSpace(repoPath))
	if err != nil {
		return nil, err
	}
	// Keep branch selection free from stale worktree metadata.
	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[DEBUG-GIT] failed to prune worktrees before listing branches", "error", pruneErr)
	}
	return repo.ListBranchesForWorktreeBase()
}

// requireWorktreeInfo returns metadata for sessions that are backed by an
// actual worktree directory (Path must be non-empty).
// Repo-tracked sessions that only carry RepoPath/BranchName are rejected.
func (a *App) requireWorktreeInfo(sessionName string) (*tmux.SessionWorktreeInfo, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, err
	}
	worktreeInfo, err := sessions.GetWorktreeInfo(sessionName)
	if err != nil {
		return nil, err
	}
	if worktreeInfo == nil || !worktreeInfo.IsWorktreeSession() {
		return nil, fmt.Errorf("session %s has no worktree", sessionName)
	}
	return worktreeInfo, nil
}

// PromoteWorktreeToBranch promotes a detached HEAD worktree to a named branch.
func (a *App) PromoteWorktreeToBranch(sessionName string, branchName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	branchName = strings.TrimSpace(branchName)

	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}

	if err := gitpkg.ValidateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}

	worktreeInfo, err := a.requireWorktreeInfo(sessionName)
	if err != nil {
		return err
	}
	wtPath := worktreeInfo.Path
	repoPath := worktreeInfo.RepoPath
	baseBranch := worktreeInfo.BaseBranch
	isDetached := worktreeInfo.IsDetached
	if !isDetached {
		return fmt.Errorf("session %s is not a detached worktree", sessionName)
	}

	wtRepo, err := gitpkg.Open(wtPath)
	if err != nil {
		return fmt.Errorf("failed to open worktree: %w", err)
	}

	if err := wtRepo.CheckoutNewBranch(branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	if err := sessions.SetWorktreeInfo(sessionName, &tmux.SessionWorktreeInfo{
		Path:       wtPath,
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseBranch: baseBranch,
		IsDetached: false,
	}); err != nil {
		return fmt.Errorf("failed to update worktree info: %w", err)
	}

	slog.Debug("[DEBUG-GIT] worktree promoted to branch",
		"session", sessionName, "branch", branchName, "path", wtPath)

	a.emitSnapshot()
	return nil
}

// ListWorktreesByRepo returns all worktree information for a given repository.
// Stale entries (folders that no longer exist) are pruned before listing
// so the UI never shows invalid worktrees.
func (a *App) ListWorktreesByRepo(repoPath string) ([]gitpkg.WorktreeInfo, error) {
	repo, err := gitpkg.Open(strings.TrimSpace(repoPath))
	if err != nil {
		return nil, err
	}
	// NOTE: Prune failure is non-fatal; proceed with listing even if prune
	// fails so the user still sees available worktrees.
	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[DEBUG-GIT] failed to prune worktrees before listing", "error", pruneErr)
	}
	return repo.ListWorktreesWithInfo()
}

// CleanupWorktree manually removes the worktree associated with a session.
func (a *App) CleanupWorktree(sessionName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}
	worktreeInfo, err := a.requireWorktreeInfo(sessionName)
	if err != nil {
		return err
	}
	wtPath := worktreeInfo.Path
	repoPath := worktreeInfo.RepoPath

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	if err := repo.RemoveWorktree(wtPath); err != nil {
		// Try forced removal.
		if fErr := repo.RemoveWorktreeForced(wtPath); fErr != nil {
			return fmt.Errorf("failed to remove worktree (forced): %w", fErr)
		}
	}

	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[DEBUG-GIT] failed to prune worktrees after cleanup", "error", pruneErr)
	}

	a.cleanupOrphanedLocalWorktreeBranch(repo, worktreeInfo.BranchName)

	// Clear worktree metadata.
	return sessions.SetWorktreeInfo(sessionName, nil)
}

// runSetupScripts executes configured setup scripts in the worktree directory.
// This runs asynchronously; results are emitted via Wails events.
//
// SECURITY: See WorktreeConfig in internal/config/config.go for the trust
// boundary of setup_scripts and copy_files.
func (a *App) runSetupScripts(wtPath, sessionName, shell string, scripts []string) {
	const setupTimeout = 5 * time.Minute
	if strings.TrimSpace(shell) == "" {
		shell = "powershell.exe"
	}

	// Use app context as parent so scripts are cancelled on app shutdown.
	// If app context is nil (startup race), fall back to Background; each script
	// still has its own setupTimeout so it will not run indefinitely.
	parentCtx := a.runtimeContext()
	if parentCtx == nil {
		parentCtx = context.Background()
		slog.Warn("[DEBUG-GIT] runSetupScripts: app context not yet available, using background context",
			"session", sessionName)
	}
	// Emit using the latest app context when available; otherwise fall back to
	// the parent context used by script execution.
	latestAppCtx := func() context.Context {
		if current := a.runtimeContext(); current != nil {
			return current
		}
		return parentCtx
	}

	for i, script := range scripts {
		script = strings.TrimSpace(script)
		if script == "" {
			continue
		}

		slog.Debug("[DEBUG-GIT] running setup script",
			"session", sessionName, "script", script, "index", i)

		ctx, cancel := context.WithTimeout(parentCtx, setupTimeout)
		shellFlag := shellExecFlag(shell)
		output, err := executeSetupCommandFn(ctx, shell, shellFlag, script, wtPath)
		cancel()

		if err != nil {
			slog.Warn("[DEBUG-GIT] setup script failed",
				"session", sessionName, "script", script,
				"error", err, "output", string(output))
			a.emitRuntimeEventWithContext(latestAppCtx(), "worktree:setup-complete", map[string]any{
				"sessionName": sessionName,
				"success":     false,
				"error":       fmt.Sprintf("script %q failed: %v", script, err),
			})
			return
		}

		slog.Debug("[DEBUG-GIT] setup script completed",
			"session", sessionName, "script", script)
	}

	a.emitRuntimeEventWithContext(latestAppCtx(), "worktree:setup-complete", map[string]any{
		"sessionName": sessionName,
		"success":     true,
	})
}

// CheckWorktreeStatus returns the worktree status for a session.
// Used by the frontend to decide what confirmation dialog to show before closing.
func (a *App) CheckWorktreeStatus(sessionName string) (WorktreeStatus, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return WorktreeStatus{}, errors.New("session name is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return WorktreeStatus{}, err
	}

	worktreeInfo, err := sessions.GetWorktreeInfo(sessionName)
	if err != nil {
		return WorktreeStatus{}, err
	}
	if worktreeInfo == nil || !worktreeInfo.IsWorktreeSession() {
		return WorktreeStatus{HasWorktree: false}, nil
	}
	wtPath := worktreeInfo.Path
	branchName := worktreeInfo.BranchName
	isDetached := worktreeInfo.IsDetached

	wtRepo, err := gitpkg.Open(wtPath)
	if err != nil {
		return WorktreeStatus{}, fmt.Errorf("failed to open worktree: %w", err)
	}

	hasUncommitted, err := wtRepo.HasUncommittedChanges()
	if err != nil {
		return WorktreeStatus{}, fmt.Errorf("failed to check uncommitted changes: %w", err)
	}

	hasUnpushed, err := wtRepo.HasUnpushedCommits()
	if err != nil {
		// Non-fatal: treat as no unpushed commits (e.g. detached HEAD has no upstream).
		slog.Debug("[DEBUG-GIT] HasUnpushedCommits failed, treating as no unpushed",
			"session", sessionName, "error", err)
		hasUnpushed = false
	}

	// Use stored branchName; fall back to git query for accuracy.
	if branchName == "" && !isDetached {
		var branchErr error
		branchName, branchErr = wtRepo.CurrentBranch()
		if branchErr != nil {
			slog.Warn("[DEBUG-GIT] failed to get current branch, leaving empty",
				"session", sessionName, "error", branchErr)
		}
	}

	return WorktreeStatus{
		HasWorktree:    true,
		HasUncommitted: hasUncommitted,
		HasUnpushed:    hasUnpushed,
		BranchName:     branchName,
		IsDetached:     isDetached,
	}, nil
}

// CommitAndPushWorktree commits and/or pushes changes in the session's worktree.
func (a *App) CommitAndPushWorktree(sessionName, commitMessage string, push bool) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	commitMessage = strings.TrimSpace(commitMessage)
	if _, err := a.requireSessions(); err != nil {
		return err
	}

	worktreeInfo, err := a.requireWorktreeInfo(sessionName)
	if err != nil {
		return err
	}
	wtPath := worktreeInfo.Path

	wtRepo, err := gitpkg.Open(wtPath)
	if err != nil {
		return fmt.Errorf("failed to open worktree: %w", err)
	}

	if commitMessage != "" {
		if err := wtRepo.CommitAll(commitMessage); err != nil {
			return fmt.Errorf("commit failed: %w", err)
		}
		slog.Debug("[DEBUG-GIT] worktree committed",
			"session", sessionName, "message", commitMessage)
	}

	if push {
		if err := wtRepo.Push(); err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		slog.Debug("[DEBUG-GIT] worktree pushed", "session", sessionName)
	}

	return nil
}

// shellExecFlag returns the command-execution flag for the given shell.
func shellExecFlag(shell string) string {
	base := strings.ToLower(filepath.Base(shell))
	switch base {
	case "cmd.exe":
		return "/c"
	case "bash.exe", "wsl.exe":
		return "-c"
	default:
		// PowerShell (powershell.exe, pwsh.exe) and unknown shells.
		return "-Command"
	}
}

// findAvailableSessionName returns name if no session with that name exists.
// Otherwise it appends -2, -3, ... until a free name is found.
func (a *App) findAvailableSessionName(name string) string {
	sessions, err := a.requireSessions()
	if err != nil {
		return name
	}
	if !sessions.HasSession(name) {
		return name
	}
	for i := 2; i <= 100; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !sessions.HasSession(candidate) {
			return candidate
		}
	}
	// Fallback: use timestamp suffix.
	return fmt.Sprintf("%s-%d", name, time.Now().UnixMilli())
}

// GetCurrentBranch returns the current branch of the repository at repoPath.
// Returns "" for detached HEAD state.
func (a *App) GetCurrentBranch(repoPath string) (string, error) {
	repo, err := gitpkg.Open(strings.TrimSpace(repoPath))
	if err != nil {
		return "", err
	}
	return repo.CurrentBranch()
}

// CreateSessionWithExistingWorktree creates a session using an existing worktree.
// No new worktree is created; the session opens in the given worktree path.
// Returns an error if the worktree path is already in use by another session.
// When enableAgentTeam is true, Agent Teams environment variables are set.
func (a *App) CreateSessionWithExistingWorktree(
	repoPath string,
	sessionName string,
	worktreePath string,
	enableAgentTeam bool,
) (snapshot tmux.SessionSnapshot, retErr error) {
	sessions, router, err := a.requireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	repoPath = strings.TrimSpace(repoPath)
	sessionName = strings.TrimSpace(sessionName)
	worktreePath = strings.TrimSpace(worktreePath)
	cfg := a.getConfigSnapshot()

	if !cfg.Worktree.Enabled {
		return tmux.SessionSnapshot{}, fmt.Errorf("worktree feature is disabled in config")
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return tmux.SessionSnapshot{}, fmt.Errorf("worktree path does not exist: %s", worktreePath)
	}

	// Prevent branch mixing: reject if another session already uses this worktree.
	if conflict := a.findSessionByWorktreePath(worktreePath); conflict != "" {
		return tmux.SessionSnapshot{}, fmt.Errorf(
			"worktree path is already in use by session %q: %s", conflict, worktreePath)
	}

	sessionName = a.findAvailableSessionName(sessionName)

	// Detect current branch of the existing worktree.
	var branchName string
	var isDetached bool
	wtRepo, err := gitpkg.Open(worktreePath)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to open worktree: %w", err)
	}
	branchName, err = currentBranchFn(wtRepo)
	if err != nil {
		// NOTE: Intentionally checking branchName even when err != nil.
		// If branchName is empty with an error, we treat the worktree as detached HEAD
		// (best-effort recovery). If branchName is non-empty with an error, the error
		// likely indicates a meaningful issue (e.g. ambiguous ref) that should be surfaced.
		if strings.TrimSpace(branchName) == "" {
			slog.Warn("[DEBUG-GIT] failed to detect current branch, treating worktree as detached",
				"path", worktreePath, "error", err)
			branchName = ""
		} else {
			return tmux.SessionSnapshot{}, fmt.Errorf("failed to detect current branch: %w", err)
		}
	}
	isDetached = branchName == ""

	createdName := ""
	defer func() {
		if retErr == nil || createdName == "" {
			return
		}
		if rollbackErr := a.rollbackCreatedSession(createdName); rollbackErr != nil {
			slog.Warn("[DEBUG-GIT] rollback kill-session failed", "session", createdName, "error", rollbackErr)
			retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackErr)
		}
		// Notify frontend after session rollback for UI consistency (#69).
		a.emitSnapshot()
	}()

	createdName, err = a.createSessionForDirectory(router, worktreePath, sessionName, enableAgentTeam)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	if err := sessions.SetWorktreeInfo(createdName, &tmux.SessionWorktreeInfo{
		Path:       worktreePath,
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseBranch: "",
		IsDetached: isDetached,
	}); err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to set worktree info: %w", err)
	}

	if err := a.storeRootPath(createdName, repoPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}
	snapshot, retErr = a.activateCreatedSession(createdName)
	return snapshot, retErr
}

// findSessionByWorktreePath returns the session name that uses the given
// worktree path, or "" if no active session uses it.
func (a *App) findSessionByWorktreePath(wtPath string) string {
	sessions, err := a.requireSessions()
	if err != nil {
		return ""
	}
	normalizedPath := filepath.Clean(wtPath)
	snapshots := sessions.Snapshot()
	for _, s := range snapshots {
		if s.Worktree != nil && s.Worktree.Path != "" && pathsEqualFold(s.Worktree.Path, normalizedPath) {
			return s.Name
		}
	}
	return ""
}

// CheckWorktreePathConflict checks whether the given worktree path is already
// used by an active session. Returns the session name if conflict exists, or "".
func (a *App) CheckWorktreePathConflict(worktreePath string) string {
	return a.findSessionByWorktreePath(strings.TrimSpace(worktreePath))
}
