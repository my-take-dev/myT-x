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

// executeRouterRequestFn is a test seam for router command execution.
var executeRouterRequestFn = func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
	return router.Execute(req)
}

// currentBranchFn is a test seam for resolving the current branch.
var currentBranchFn = func(repo *gitpkg.Repository) (string, error) {
	return repo.CurrentBranch()
}

// executeSetupCommandFn is a test seam for setup script execution.
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
	opts.BranchName = strings.TrimSpace(opts.BranchName)
	opts.BaseBranch = strings.TrimSpace(opts.BaseBranch)
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("session name is required")
	}
	validatedBranchName, err := validateAndTrimWorktreeBranchName(opts.BranchName)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	opts.BranchName = validatedBranchName
	cfg := a.getConfigSnapshot()
	createdName := ""
	wtPath := ""
	worktreeCreated := false
	var repo *gitpkg.Repository
	var setupScriptsCancel context.CancelFunc
	var setupScriptsDone chan struct{}
	// NOTE: Emit snapshots on both success and rollback paths so frontend
	// subscribers stay synchronized even when RPC return values are not consumed.
	defer func() {
		if retErr == nil {
			return
		}
		if setupScriptsCancel != nil {
			setupScriptsCancel()
			if !waitForSetupScriptsCancellation(setupScriptsDone, 3*time.Second) {
				slog.Warn("[WARN-GIT] timed out waiting for setup scripts to stop during rollback",
					"session", createdName, "worktree", wtPath)
			}
		}
		if createdName != "" {
			if rollbackSessionErr := a.rollbackCreatedSession(createdName); rollbackSessionErr != nil {
				retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackSessionErr)
			}
			// Notify frontend after session rollback for UI consistency (#69).
			a.requestSnapshot(true)
		}
		if worktreeCreated && repo != nil && wtPath != "" {
			if rollbackErr := rollbackWorktree(repo, wtPath, opts.BranchName); rollbackErr != nil {
				eventSessionName := strings.TrimSpace(createdName)
				if eventSessionName == "" {
					eventSessionName = strings.TrimSpace(sessionName)
				}
				a.emitWorktreeCleanupFailure(eventSessionName, wtPath, rollbackErr)
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

	// Deduplicate session name BEFORE creating the worktree to avoid
	// orphaned branches when session creation fails due to name collision.
	sessionName = a.findAvailableSessionName(sessionName)

	repo, err = gitpkg.Open(repoPath)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to open repository: %w", err)
	}

	resolvedBaseBranch := ""
	wtPath, resolvedBaseBranch, err = a.createWorktreeForSession(repo, repoPath, sessionName, opts)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	worktreeCreated = true

	createdName, err = a.createSessionForDirectory(router, wtPath, sessionName, opts.EnableAgentTeam)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Store worktree metadata on the session.
	if err := sessions.SetWorktreeInfo(createdName, &tmux.SessionWorktreeInfo{
		Path:       wtPath,
		RepoPath:   repoPath,
		BranchName: opts.BranchName,
		BaseBranch: resolvedBaseBranch,
		IsDetached: false,
	}); err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to set worktree info: %w", err)
	}

	if err := a.storeRootPath(createdName, repoPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Copy configured files (e.g. .env) from repo to worktree.
	if copyFailures := copyConfigFilesToWorktree(repoPath, wtPath, cfg.Worktree.CopyFiles); len(copyFailures) > 0 {
		slog.Warn("[WARN-GIT] failed to copy one or more configured files to worktree",
			"session", createdName, "path", wtPath, "files", copyFailures)
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
		parentCtx := context.Background()
		if appCtx := a.runtimeContext(); appCtx != nil {
			parentCtx = appCtx
		}
		setupScriptsCtx, cancel := context.WithCancel(parentCtx)
		setupScriptsCancel = cancel
		setupScriptsDone = make(chan struct{})
		a.setupWG.Add(1)
		go func(ctx context.Context, cancel context.CancelFunc, done chan struct{}) {
			defer close(done)
			defer a.setupWG.Done()
			defer cancel()
			defer func() {
				recoverBackgroundPanic("worktree-setup-scripts", recover())
			}()
			a.runSetupScriptsWithParentContext(ctx, wtPath, createdName, cfg.Shell, cfg.Worktree.SetupScripts)
		}(setupScriptsCtx, cancel, setupScriptsDone)
	}

	snapshot, retErr = a.activateCreatedSession(createdName)
	if retErr == nil {
		a.requestSnapshot(true)
	}
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
	createdName := strings.TrimSpace(resp.Stdout)
	if createdName == "" {
		// tmux can occasionally return empty stdout even when session creation
		// succeeded. Attempt cleanup by the requested -s name to avoid orphaning.
		if rollbackErr := rollbackSessionByRouter(router, sessionName); rollbackErr != nil {
			return "", fmt.Errorf("failed to create session: empty session name returned by tmux (rollback also failed: %v)", rollbackErr)
		}
		return "", errors.New("failed to create session: empty session name returned by tmux")
	}
	return createdName, nil
}

func validateAndTrimWorktreeBranchName(branchName string) (string, error) {
	normalized := strings.TrimSpace(branchName)
	if normalized == "" {
		return "", fmt.Errorf("branch name is required for new worktree creation")
	}
	if err := gitpkg.ValidateBranchName(normalized); err != nil {
		return "", fmt.Errorf("invalid branch name: %w", err)
	}
	return normalized, nil
}

func chooseWorktreeIdentifier(branchName, sessionName string) string {
	identifier := gitpkg.SanitizeCustomName(branchName)
	if identifier != "work" {
		return identifier
	}

	identifier = gitpkg.SanitizeCustomName(sessionName)
	if identifier != "work" {
		return identifier
	}

	return fmt.Sprintf("wt-%d", time.Now().UnixMilli())
}

// createWorktreeForSession creates the git worktree for a new session.
// Handles pull, path generation, validation, and the actual worktree creation.
func (a *App) createWorktreeForSession(
	repo *gitpkg.Repository, repoPath, sessionName string, opts WorktreeSessionOptions,
) (wtPath string, resolvedBaseBranch string, err error) {
	// BranchName is validated once in CreateSessionWithWorktree before this helper is called.
	branchName := opts.BranchName
	if branchName == "" {
		return "", "", errors.New("branch name is required for new worktree creation")
	}

	if opts.PullBeforeCreate {
		if pullErr := repo.Pull(); pullErr != nil {
			slog.Warn("[WARN-GIT] pull before worktree creation failed",
				"error", pullErr, "repoPath", repoPath)
			return "", "", fmt.Errorf("failed to pull latest changes: %w", pullErr)
		}
	}

	identifier := chooseWorktreeIdentifier(branchName, sessionName)

	wtPath = gitpkg.FindAvailableWorktreePath(gitpkg.GenerateWorktreePath(repoPath, identifier))

	if err := gitpkg.ValidateWorktreePath(wtPath); err != nil {
		return "", "", fmt.Errorf("invalid worktree path: %w", err)
	}

	wtDir := gitpkg.GenerateWorktreeDirPath(repoPath)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", "", fmt.Errorf("failed to create worktree directory %s: %w", wtDir, err)
	}

	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		// Resolve actual branch name for display purposes.
		if resolved, brErr := repo.CurrentBranch(); brErr == nil && resolved != "" {
			baseBranch = resolved
		} else {
			if brErr != nil {
				slog.Warn("[WARN-GIT] failed to detect current branch, falling back to HEAD",
					"path", repoPath, "error", brErr)
			}
			baseBranch = "HEAD"
		}
	}

	if err := repo.CreateWorktree(wtPath, branchName, baseBranch); err != nil {
		return "", "", fmt.Errorf("failed to create worktree: %w", err)
	}

	slog.Debug("[DEBUG-GIT] worktree created",
		"path", wtPath, "repoPath", repoPath, "detached", false)

	return wtPath, baseBranch, nil
}

// rollbackWorktree removes a worktree and prunes orphaned entries.
// Returns the removal error (if any) for inclusion in the caller's error message.
func rollbackWorktree(repo *gitpkg.Repository, wtPath, branchName string) error {
	var rollbackErr error
	if rmErr := repo.RemoveWorktreeForced(wtPath); rmErr != nil {
		slog.Warn("[WARN-GIT] failed to rollback worktree", "error", rmErr)
		rollbackErr = fmt.Errorf("failed to remove worktree during rollback: %w", rmErr)
	}
	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[WARN-GIT] failed to prune worktrees during rollback", "error", pruneErr)
	}
	branchName = strings.TrimSpace(branchName)
	if branchName != "" {
		if _, cleanupErr := repo.CleanupLocalBranchIfOrphaned(branchName); cleanupErr != nil {
			slog.Warn("[WARN-GIT] failed to cleanup branch during rollback",
				"branch", branchName, "error", cleanupErr)
			if rollbackErr == nil {
				rollbackErr = fmt.Errorf("failed to cleanup rollback branch %q: %w", branchName, cleanupErr)
			}
		}
	}
	return rollbackErr
}

func waitForSetupScriptsCancellation(done <-chan struct{}, timeout time.Duration) bool {
	if done == nil {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// copyConfigFilesToWorktree copies configured files (e.g. .env) from the
// repository root to the worktree. Returns a list of files that failed to copy.
// Missing source files are silently skipped (common for optional files like .env).
func copyConfigFilesToWorktree(repoPath, wtPath string, files []string) []string {
	var failures []string
	repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoPath)
	if repoErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve repository base path for copy",
			"repoPath", repoPath, "error", repoErr)
		return normalizeCopyFailures(files)
	}
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtPath)
	if wtErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve worktree base path for copy",
			"worktreePath", wtPath, "error", wtErr)
		return normalizeCopyFailures(files)
	}
	for _, file := range files {
		if failed := copyConfigFileToWorktree(repoBase, wtBase, file); failed {
			failures = append(failures, file)
		}
	}
	return failures
}

func copyConfigFileToWorktree(repoBase, wtBase, file string) bool {
	cleaned := filepath.Clean(file)
	if filepath.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, "..") {
		slog.Warn("[WARN-GIT] skipping unsafe copy_files entry", "file", file)
		return false
	}

	src := filepath.Join(repoBase, cleaned)
	dst := filepath.Join(wtBase, cleaned)
	if !isPathWithinBase(src, repoBase) || !isPathWithinBase(dst, wtBase) {
		slog.Warn("[WARN-GIT] skipping copy_files entry escaping base directory", "file", file)
		return false
	}

	resolvedSrc, resolveSrcErr := filepath.EvalSymlinks(src)
	if resolveSrcErr != nil {
		if !os.IsNotExist(resolveSrcErr) {
			slog.Warn("[WARN-GIT] failed to resolve source file symlink for copy",
				"src", src, "error", resolveSrcErr)
			return true
		}
		return false
	}
	if !isPathWithinBase(resolvedSrc, repoBase) {
		slog.Warn("[WARN-GIT] skipping copy_files entry escaping repository via symlink",
			"file", file, "resolvedSrc", resolvedSrc)
		return false
	}

	data, readErr := os.ReadFile(resolvedSrc)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			slog.Warn("[WARN-GIT] failed to read source file for copy",
				"src", resolvedSrc, "error", readErr)
			return true
		}
		return false
	}

	canWrite, failed := validateCopyDestination(dst, wtBase, file)
	if failed {
		return true
	}
	if !canWrite {
		return false
	}

	if writeErr := os.WriteFile(dst, data, 0o600); writeErr != nil {
		slog.Warn("[WARN-GIT] failed to copy file to worktree",
			"src", resolvedSrc, "dst", dst, "error", writeErr)
		return true
	}
	return false
}

func validateCopyDestination(dst, wtBase, file string) (canWrite bool, failed bool) {
	if dstDir := filepath.Dir(dst); dstDir != "." {
		if mkErr := os.MkdirAll(dstDir, 0o755); mkErr != nil {
			slog.Warn("[WARN-GIT] failed to create destination directory",
				"dir", dstDir, "error", mkErr)
			return false, true
		}
		resolvedDstDir, resolveDstDirErr := filepath.EvalSymlinks(dstDir)
		if resolveDstDirErr != nil {
			slog.Warn("[WARN-GIT] failed to resolve destination directory symlink for copy",
				"dir", dstDir, "error", resolveDstDirErr)
			return false, true
		}
		if !isPathWithinBase(resolvedDstDir, wtBase) {
			slog.Warn("[WARN-GIT] skipping copy_files entry escaping worktree via symlink",
				"file", file, "resolvedDstDir", resolvedDstDir)
			return false, false
		}
	}

	if info, lstatErr := os.Lstat(dst); lstatErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolvedDst, resolveDstErr := filepath.EvalSymlinks(dst)
			if resolveDstErr != nil {
				slog.Warn("[WARN-GIT] failed to resolve destination file symlink for copy",
					"dst", dst, "error", resolveDstErr)
				return false, true
			}
			if !isPathWithinBase(resolvedDst, wtBase) {
				slog.Warn("[WARN-GIT] skipping copy_files entry writing outside worktree via symlink",
					"file", file, "resolvedDst", resolvedDst)
				return false, false
			}
			return true, false
		}
		if info.Mode().IsRegular() {
			slog.Warn("[WARN-GIT] overwriting existing destination file from copy_files",
				"file", file, "dst", dst)
		}
	} else if !os.IsNotExist(lstatErr) {
		slog.Warn("[WARN-GIT] failed to inspect destination file before copy",
			"dst", dst, "error", lstatErr)
		return false, true
	}
	return true, false
}

func normalizeCopyFailures(files []string) []string {
	var failures []string
	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		failures = append(failures, trimmed)
	}
	return failures
}

func resolveSymlinkEvaluatedBasePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink path: %w", err)
	}
	return filepath.Clean(resolvedPath), nil
}

func isPathWithinBase(path, base string) bool {
	relPath, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return false
	}
	return true
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
	// NOTE: PruneWorktrees is not needed here because ListBranchesForWorktreeBase
	// reads git refs directly and does not depend on worktree metadata.
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
		if rollbackErr := rollbackPromotedWorktreeBranch(wtRepo, branchName); rollbackErr != nil {
			return fmt.Errorf("failed to update worktree info: %w (git rollback also failed: %v)", err, rollbackErr)
		}
		return fmt.Errorf("failed to update worktree info: %w", err)
	}

	slog.Debug("[DEBUG-GIT] worktree promoted to branch",
		"session", sessionName, "branch", branchName, "path", wtPath)

	a.requestSnapshot(true)
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
		slog.Warn("[WARN-GIT] failed to prune worktrees before listing", "error", pruneErr)
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
		slog.Warn("[WARN-GIT] normal worktree removal failed, trying forced removal",
			"session", sessionName, "path", wtPath, "error", err)
		// Try forced removal.
		if fErr := repo.RemoveWorktreeForced(wtPath); fErr != nil {
			return fmt.Errorf("failed to remove worktree (forced): %w", fErr)
		}
	}

	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[WARN-GIT] failed to prune worktrees after cleanup", "error", pruneErr)
	}

	a.cleanupOrphanedLocalWorktreeBranch(repo, worktreeInfo.BranchName)

	// Clear worktree metadata.
	return sessions.SetWorktreeInfo(sessionName, nil)
}

func (a *App) runSetupScriptsWithParentContext(parentCtx context.Context, wtPath, sessionName, shell string, scripts []string) {
	const setupTimeout = 5 * time.Minute
	if strings.TrimSpace(shell) == "" {
		shell = "powershell.exe"
	}

	// If parent context is not provided, use app context so scripts are cancelled
	// on app shutdown. When app context is nil (startup race), fall back to
	// Background; each script still has setupTimeout so it will not run forever.
	if parentCtx == nil {
		parentCtx = a.runtimeContext()
		if parentCtx == nil {
			parentCtx = context.Background()
			slog.Warn("[WARN-GIT] runSetupScripts: app context not yet available, using background context",
				"session", sessionName)
		}
	}
	// Emit using the latest app context when available; otherwise fall back to
	// the parent context used by script execution.
	latestAppCtx := func() context.Context {
		if current := a.runtimeContext(); current != nil {
			return current
		}
		return parentCtx
	}
	shellFlag := shellExecFlag(shell)

	for i, script := range scripts {
		script = strings.TrimSpace(script)
		if script == "" {
			continue
		}

		slog.Debug("[DEBUG-GIT] running setup script",
			"session", sessionName, "script", script, "index", i)

		ctx, cancel := context.WithTimeout(parentCtx, setupTimeout)
		output, err := executeSetupCommandFn(ctx, shell, shellFlag, script, wtPath)
		cancel()

		if err != nil {
			slog.Warn("[WARN-GIT] setup script failed",
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
			slog.Warn("[WARN-GIT] failed to get current branch, leaving empty",
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
	if commitMessage == "" && push {
		// NOTE: Empty commit message with push=true is allowed by design.
		// In this mode, the API pushes existing local commits without creating
		// a new commit.
		slog.Debug("[DEBUG-GIT] push requested without commit message; pushing existing commits only",
			"session", sessionName)
	}
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

const maxSessionNameSuffix = 100

// findAvailableSessionName returns name if no session with that name exists.
// Otherwise it appends -2, -3, ... until a free name is found.
func (a *App) findAvailableSessionName(name string) string {
	sessions, err := a.requireSessions()
	if err != nil {
		slog.Debug("[DEBUG-SESSION] findAvailableSessionName fallback to original name",
			"name", name, "error", err)
		return name
	}
	if !sessions.HasSession(name) {
		return name
	}
	for i := 2; i <= maxSessionNameSuffix; i++ {
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
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("session name is required")
	}
	if repoPath == "" {
		return tmux.SessionSnapshot{}, errors.New("repository path is required")
	}
	if worktreePath == "" {
		return tmux.SessionSnapshot{}, errors.New("worktree path is required")
	}
	cfg := a.getConfigSnapshot()

	if !cfg.Worktree.Enabled {
		return tmux.SessionSnapshot{}, fmt.Errorf("worktree feature is disabled in config")
	}

	if _, err := os.Stat(worktreePath); err != nil {
		if os.IsNotExist(err) {
			return tmux.SessionSnapshot{}, fmt.Errorf("worktree path does not exist: %s", worktreePath)
		}
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to stat worktree path %s: %w", worktreePath, err)
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
			slog.Warn("[WARN-GIT] failed to detect current branch, treating worktree as detached",
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
			slog.Warn("[WARN-GIT] rollback kill-session failed", "session", createdName, "error", rollbackErr)
			retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackErr)
		}
		// Notify frontend after session rollback for UI consistency (#69).
		a.requestSnapshot(true)
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
	if retErr == nil {
		a.requestSnapshot(true)
	}
	return snapshot, retErr
}

func rollbackPromotedWorktreeBranch(repo *gitpkg.Repository, branchName string) error {
	var checkoutErr error
	if err := repo.CheckoutDetachedHead(); err != nil {
		checkoutErr = fmt.Errorf("failed to restore detached HEAD during promotion rollback: %w", err)
	}

	var deleteErr error
	if err := repo.DeleteLocalBranch(branchName, true); err != nil {
		deleteErr = fmt.Errorf("failed to delete promoted branch %q during rollback: %w", branchName, err)
	}

	switch {
	case checkoutErr != nil && deleteErr != nil:
		return fmt.Errorf("%w; %w", checkoutErr, deleteErr)
	case checkoutErr != nil:
		return checkoutErr
	case deleteErr != nil:
		return deleteErr
	default:
		return nil
	}
}

// findSessionByWorktreePath returns the session name that uses the given
// worktree path, or "" if no active session uses it.
func (a *App) findSessionByWorktreePath(wtPath string) string {
	sessions, err := a.requireSessions()
	if err != nil {
		// During startup/shutdown, session manager access can fail transiently.
		// Treat this as "no conflict" to avoid blocking worktree-attach flows.
		slog.Debug("[DEBUG-GIT] findSessionByWorktreePath fallback to no-conflict",
			"path", wtPath, "error", err)
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
