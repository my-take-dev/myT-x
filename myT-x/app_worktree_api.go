package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
//   - BranchName is required (non-empty) -> Named branch mode
//   - BaseBranch == ""  -> Uses current HEAD as the base for the worktree
//
// NOTE: Detached HEAD mode was removed from the UI. Existing detached worktrees
// (via CreateSessionWithExistingWorktree) are still supported and can be
// promoted via PromoteWorktreeToBranch.
type WorktreeSessionOptions struct {
	BranchName       string `json:"branch_name"`        // required: branch name for the new worktree
	BaseBranch       string `json:"base_branch"`        // empty = current HEAD
	PullBeforeCreate bool   `json:"pull_before_create"` // pull latest before creating worktree
	EnableAgentTeam  bool   `json:"enable_agent_team"`  // set Agent Teams env vars on initial pane
	UseClaudeEnv     bool   `json:"use_claude_env"`     // apply claude_env config to panes
	UsePaneEnv       bool   `json:"use_pane_env"`       // apply pane_env config to additional panes
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

	createdName, err = a.createSessionForDirectory(router, wtPath, sessionName, CreateSessionOptions{
		EnableAgentTeam: opts.EnableAgentTeam,
		UseClaudeEnv:    opts.UseClaudeEnv,
		UsePaneEnv:      opts.UsePaneEnv,
	})
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Set session-level env flags before any additional pane can be created.
	applySessionEnvFlags(sessions, createdName, opts.UseClaudeEnv, opts.UsePaneEnv)

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

	// Copy configured directories (e.g. .vscode) from repo to worktree.
	if copyDirFailures := copyConfigDirsToWorktree(repoPath, wtPath, cfg.Worktree.CopyDirs); len(copyDirFailures) > 0 {
		slog.Warn("[WARN-GIT] failed to copy one or more configured directories to worktree",
			"session", createdName, "path", wtPath, "dirs", copyDirFailures)
		a.emitRuntimeEvent("worktree:copy-dirs-failed", map[string]any{
			"sessionName": createdName,
			"dirs":        copyDirFailures,
		})
	}

	// NOTE: Setup scripts run regardless of copy failures because they are
	// independent operations. Copy files/dirs are best-effort;
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

// createSessionForDirectory creates a tmux session rooted at sessionDir.
//
// DESIGN NOTE: opts.UsePaneEnv is not directly referenced in this function.
// It is intentionally forwarded to callers who apply it via applySessionEnvFlags
// after session creation succeeds. This separation keeps session creation
// (tmux new-session) decoupled from session-level flag storage.
func (a *App) createSessionForDirectory(
	router *tmux.CommandRouter,
	sessionDir,
	sessionName string,
	opts CreateSessionOptions,
) (string, error) {
	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-c": sessionDir,
			"-s": sessionName,
		},
	}
	if opts.EnableAgentTeam {
		req.Env = agentTeamEnvVars(sessionName)
	}

	// Merge claude_env into initial pane env when enabled.
	//
	// DESIGN NOTE (initial pane vs additional pane env merge):
	// Initial pane reads cfg.ClaudeEnv.Vars directly from the config snapshot
	// (fill-only merge: agent_team env takes priority). This is a point-in-time
	// capture at session creation; pane_env is NOT applied to the initial pane.
	//
	// Additional panes use CommandRouter.buildPaneEnvForSession() which reads
	// claudeEnvView() (runtime-updated values via UpdateClaudeEnv) and applies
	// pane_env with overwrite semantics when both flags are enabled.
	//
	// This divergence is intentional:
	//   - Initial pane: config snapshot for deterministic session bootstrap
	//   - Additional panes: latest runtime values for hot-reloaded env changes
	// If the merge rule for initial panes needs to change, update this block
	// independently of buildPaneEnvForSession to avoid unintended side effects.
	if opts.UseClaudeEnv {
		cfg := a.getConfigSnapshot()
		if cfg.ClaudeEnv != nil && len(cfg.ClaudeEnv.Vars) > 0 {
			if req.Env == nil {
				req.Env = make(map[string]string)
			}
			for k, v := range cfg.ClaudeEnv.Vars {
				if _, exists := req.Env[k]; !exists {
					req.Env[k] = v // fill-only (agent team env takes priority)
				}
			}
		}
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

var (
	// copy_dirs guardrails prevent accidental large recursive copies from
	// blocking worktree creation or exhausting disk space.
	// Limits are enforced across all configured copy_dirs entries in one run.
	// Kept as vars to allow deterministic limit tests via save/restore.
	maxCopyDirsFileCount        = 10_000
	maxCopyDirsTotalBytes int64 = 500 * 1024 * 1024
	// NOTE: Package-level seams are process-global and not safe for t.Parallel.
	// Tests that override these vars must run sequentially.

	// walkDirFn is a test seam for directory walk behavior.
	walkDirFn = filepath.WalkDir

	// streamCopyFn is a test seam for file streaming copy behavior.
	streamCopyFn = io.Copy

	// syncFileFn is a test seam for destination fsync behavior.
	syncFileFn = func(file *os.File) error {
		return file.Sync()
	}

	// statFileInfoFn is a test seam for file metadata lookups.
	statFileInfoFn = os.Stat

	// removeFileFn is a test seam for destination cleanup behavior.
	removeFileFn = os.Remove
)

type copyWalkBudget struct {
	fileCount int
	totalSize int64
}

func copyConfigEntriesToWorktree(
	repoPath, wtPath string,
	entries []string,
	entryKind string,
	copyFn func(repoBase, wtBase, entry string) bool,
) []string {
	var failures []string
	if len(entries) == 0 {
		return failures
	}
	repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoPath)
	if repoErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve repository base path for copy",
			"repoPath", repoPath, "entryKind", entryKind, "error", repoErr)
		return normalizeCopyFailures(entries)
	}
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtPath)
	if wtErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve worktree base path for copy",
			"worktreePath", wtPath, "entryKind", entryKind, "error", wtErr)
		return normalizeCopyFailures(entries)
	}
	for _, entry := range entries {
		if failed := copyFn(repoBase, wtBase, entry); failed {
			failures = append(failures, entry)
		}
	}
	return failures
}

// copyConfigFilesToWorktree copies configured files (e.g. .env) from the
// repository root to the worktree. Returns a list of files that failed to copy.
// Missing source files are silently skipped (common for optional files like .env).
func copyConfigFilesToWorktree(repoPath, wtPath string, files []string) []string {
	return copyConfigEntriesToWorktree(repoPath, wtPath, files, "file", copyConfigFileToWorktree)
}

func validateAndResolveSourceEntry(
	repoBase, wtBase, entry, configKey, fieldKey string,
) (resolvedSrc, dstPath string, canProcess, failed bool) {
	cleaned := filepath.Clean(entry)
	if filepath.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, "..") {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping unsafe %s entry", configKey), fieldKey, entry)
		return "", "", false, true
	}

	srcPath := filepath.Join(repoBase, cleaned)
	dstPath = filepath.Join(wtBase, cleaned)
	if !isPathWithinBase(srcPath, repoBase) || !isPathWithinBase(dstPath, wtBase) {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry escaping base directory", configKey), fieldKey, entry)
		return "", "", false, true
	}

	var resolveSrcErr error
	resolvedSrc, resolveSrcErr = filepath.EvalSymlinks(srcPath)
	if resolveSrcErr != nil {
		if !errors.Is(resolveSrcErr, os.ErrNotExist) {
			slog.Warn("[WARN-GIT] failed to resolve source symlink for copy",
				"src", srcPath, "error", resolveSrcErr)
			return "", "", false, true
		}
		// Source does not exist — silent skip for optional entries.
		return "", "", false, false
	}
	if !isPathWithinBase(resolvedSrc, repoBase) {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry escaping repository via symlink", configKey),
			fieldKey, entry, "resolvedSrc", resolvedSrc)
		return "", "", false, true
	}
	return resolvedSrc, dstPath, true, false
}

func copyConfigFileToWorktree(repoBase, wtBase, file string) bool {
	resolvedSrc, dst, canProcess, failed := validateAndResolveSourceEntry(
		repoBase, wtBase, file, "copy_files", "file",
	)
	if failed {
		return true
	}
	if !canProcess {
		return false
	}

	canWrite, failed := validateCopyDestination(dst, wtBase, file, "copy_files", "file")
	if failed {
		return true
	}
	if !canWrite {
		return false
	}

	// Note: a TOCTOU window exists between destination validation and file open.
	// This is acceptable because copy paths come from trusted local configuration.
	if copyErr := copyFileByStreaming(resolvedSrc, dst); copyErr != nil {
		if errors.Is(copyErr, os.ErrNotExist) {
			slog.Debug("[DEBUG-GIT] source file disappeared before copy_files stream copy, skipping",
				"src", resolvedSrc, "dst", dst)
			return false
		}
		slog.Warn("[WARN-GIT] failed to copy file to worktree",
			"src", resolvedSrc, "dst", dst, "error", copyErr)
		return true
	}
	return false
}

func validateCopyDestination(dst, wtBase, entry, configKey, fieldKey string) (canWrite bool, failed bool) {
	if dstDir := filepath.Dir(dst); dstDir != "." {
		if !ensureDirWithinBase(dstDir, wtBase, entry, configKey, fieldKey) {
			return false, true
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
				slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry writing outside worktree via symlink", configKey),
					fieldKey, entry, "resolvedDst", resolvedDst)
				return false, true
			}
			return true, false
		}
		if info.IsDir() {
			slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry because destination is an existing directory", configKey),
				fieldKey, entry, "dst", dst)
			return false, true
		}
		if !info.Mode().IsRegular() {
			slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry because destination is not a regular file", configKey),
				fieldKey, entry, "dst", dst, "mode", info.Mode())
			return false, true
		}
		slog.Warn(fmt.Sprintf("[WARN-GIT] overwriting existing destination file from %s", configKey),
			fieldKey, entry, "dst", dst)
	} else if !errors.Is(lstatErr, os.ErrNotExist) {
		slog.Warn("[WARN-GIT] failed to inspect destination file before copy",
			"dst", dst, "error", lstatErr)
		return false, true
	}
	return true, false
}

func ensureDirWithinBase(dirPath, basePath, entry, configKey, fieldKey string) bool {
	if mkErr := os.MkdirAll(dirPath, 0o755); mkErr != nil {
		slog.Warn("[WARN-GIT] failed to create destination directory",
			"dir", dirPath, "error", mkErr)
		return false
	}
	resolvedDir, resolveDirErr := filepath.EvalSymlinks(dirPath)
	if resolveDirErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve destination directory symlink for copy",
			"dir", dirPath, "error", resolveDirErr)
		return false
	}
	if !isPathWithinBase(resolvedDir, basePath) {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry escaping worktree via symlink", configKey),
			fieldKey, entry, "resolvedDstDir", resolvedDir)
		return false
	}
	return true
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

// copyConfigDirsToWorktree copies configured directories from the
// repository root to the worktree. Returns a list of dirs that failed to copy.
// Missing source directories are silently skipped (common for optional directories).
func copyConfigDirsToWorktree(repoPath, wtPath string, dirs []string) []string {
	sharedBudget := &copyWalkBudget{}
	return copyConfigEntriesToWorktree(
		repoPath,
		wtPath,
		dirs,
		"directory",
		func(repoBase, wtBase, dir string) bool {
			return copyConfigDirToWorktreeWithBudget(repoBase, wtBase, dir, sharedBudget)
		},
	)
}

func copyConfigDirToWorktreeWithBudget(repoBase, wtBase, dir string, budget *copyWalkBudget) bool {
	if budget == nil {
		// Defensive fallback for direct unit tests and future callers.
		budget = &copyWalkBudget{}
	}

	resolvedSrc, dstDir, canProcess, failed := validateAndResolveSourceEntry(
		repoBase, wtBase, dir, "copy_dirs", "dir",
	)
	if failed {
		return true
	}
	if !canProcess {
		return false
	}

	// Verify source is actually a directory.
	srcInfo, statErr := statFileInfoFn(resolvedSrc)
	if statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			slog.Warn("[WARN-GIT] failed to stat source directory for copy",
				"src", resolvedSrc, "error", statErr)
			return true
		}
		return false
	}
	if !srcInfo.IsDir() {
		// Entry points to a regular file, not a directory — skip silently.
		slog.Debug("[DEBUG-GIT] copy_dirs entry is not a directory, skipping",
			"dir", dir, "src", resolvedSrc)
		return false
	}

	// hadError tracks whether any error occurred during walk (monotonically set to true).
	hadError := false
	walkErr := walkDirFn(resolvedSrc, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("[WARN-GIT] walk error in copy_dirs",
				"path", path, "error", err)
			hadError = true
			// Continue walking remaining entries.
			return nil
		}

		// Compute relative path from resolved source root.
		relPath, relErr := filepath.Rel(resolvedSrc, path)
		if relErr != nil {
			slog.Warn("[WARN-GIT] failed to compute relative path in copy_dirs",
				"path", path, "error", relErr)
			hadError = true
			return nil
		}

		targetPath := filepath.Join(dstDir, relPath)

		// SECURITY: validate target stays within worktree.
		if !isPathWithinBase(targetPath, wtBase) {
			slog.Warn("[WARN-GIT] skipping copy_dirs entry escaping worktree",
				"dir", dir, "targetPath", targetPath)
			hadError = true
			return nil
		}

		// Handle symlinks: resolve and check containment.
		if d.Type()&os.ModeSymlink != 0 {
			return handleSymlinkInWalk(path, targetPath, repoBase, wtBase, dir, &hadError, budget)
		}

		if d.IsDir() {
			if !ensureDirWithinBase(targetPath, wtBase, dir, "copy_dirs", "dir") {
				hadError = true
			}
			return nil
		}

		if !d.Type().IsRegular() {
			// Skip special files (devices, sockets, etc.).
			slog.Debug("[DEBUG-GIT] skipping non-regular file in copy_dirs",
				"path", path, "type", d.Type())
			return nil
		}

		fileInfo, infoErr := d.Info()
		if infoErr != nil {
			slog.Warn("[WARN-GIT] failed to read file metadata in copy_dirs",
				"path", path, "error", infoErr)
			hadError = true
			return nil
		}
		canCopy, budgetErr := reserveCopyWalkBudget(budget, fileInfo.Size(), dir, path, &hadError)
		if budgetErr != nil {
			return budgetErr
		}
		if !canCopy {
			return nil
		}

		return copyFileInWalk(path, targetPath, wtBase, dir, &hadError)
	})

	if walkErr != nil {
		slog.Warn("[WARN-GIT] directory walk failed in copy_dirs",
			"dir", dir, "error", walkErr)
		return true
	}
	return hadError
}

// handleSymlinkInWalk resolves a symlink encountered during directory walk,
// validates containment, and copies the target content.
func handleSymlinkInWalk(path, targetPath, repoBase, wtBase, dirEntry string, hadError *bool, budget *copyWalkBudget) error {
	if budget == nil {
		slog.Warn("[WARN-GIT] missing budget in copy_dirs symlink handling", "path", path)
		*hadError = true
		return nil
	}
	resolvedLink, linkErr := filepath.EvalSymlinks(path)
	if linkErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve symlink in copy_dirs",
			"path", path, "error", linkErr)
		*hadError = true
		return nil
	}
	if !isPathWithinBase(resolvedLink, repoBase) {
		slog.Debug("[DEBUG-GIT] skipping symlink escaping repository in copy_dirs",
			"path", path, "resolvedLink", resolvedLink)
		// Skip without counting as failure — intentional symlink outside repo.
		return nil
	}
	linkInfo, linkStatErr := statFileInfoFn(resolvedLink)
	if linkStatErr != nil {
		slog.Warn("[WARN-GIT] failed to stat resolved symlink in copy_dirs",
			"path", resolvedLink, "error", linkStatErr)
		*hadError = true
		return nil
	}
	if linkInfo.IsDir() {
		// Create directory in worktree for symlinked directory.
		// NOTE: Contents of the symlinked directory are intentionally NOT recursed.
		// filepath.WalkDir does not follow symlinks, so this creates an empty
		// directory shell. This is a safety measure to prevent infinite loops
		// from circular symlinks and unexpected data exposure.
		if !ensureDirWithinBase(targetPath, wtBase, dirEntry, "copy_dirs", "dir") {
			*hadError = true
		} else {
			slog.Info("[INFO-GIT] created empty directory shell for symlinked directory in copy_dirs",
				"path", path, "resolvedLink", resolvedLink, "targetPath", targetPath)
		}
		return nil
	}
	if !linkInfo.Mode().IsRegular() {
		slog.Debug("[DEBUG-GIT] skipping non-regular symlink target in copy_dirs",
			"path", resolvedLink, "mode", linkInfo.Mode())
		return nil
	}
	// Symlink points to a regular file — copy it.
	canCopy, budgetErr := reserveCopyWalkBudget(budget, linkInfo.Size(), dirEntry, resolvedLink, hadError)
	if budgetErr != nil {
		return budgetErr
	}
	if !canCopy {
		return nil
	}
	return copyFileInWalk(resolvedLink, targetPath, wtBase, dirEntry, hadError)
}

// copyFileInWalk copies a single file during directory walk.
// Updates hadError on failure. Returns nil to continue walking.
func copyFileInWalk(srcPath, dstPath, wtBase, dirEntry string, hadError *bool) error {
	// Note: a TOCTOU window exists between destination validation and file open.
	// This is acceptable because copy paths come from trusted local configuration.
	canWrite, failed := validateCopyDestination(dstPath, wtBase, dirEntry, "copy_dirs", "dir")
	if failed {
		*hadError = true
		return nil
	}
	if !canWrite {
		return nil
	}

	if copyErr := copyFileByStreaming(srcPath, dstPath); copyErr != nil {
		if errors.Is(copyErr, os.ErrNotExist) {
			slog.Debug("[DEBUG-GIT] source file disappeared during copy_dirs walk, skipping",
				"src", srcPath, "dst", dstPath)
			return nil
		}
		slog.Warn("[WARN-GIT] failed to copy file in copy_dirs",
			"src", srcPath, "dst", dstPath, "error", copyErr)
		*hadError = true
	}
	return nil
}

func reserveCopyWalkBudget(
	budget *copyWalkBudget,
	fileSize int64,
	dirEntry string,
	srcPath string,
	hadError *bool,
) (canCopy bool, walkErr error) {
	if fileSize < 0 {
		slog.Warn("[WARN-GIT] skipping copy_dirs entry with invalid file size",
			"dir", dirEntry, "path", srcPath, "size", fileSize)
		*hadError = true
		return false, nil
	}
	nextFileCount := budget.fileCount + 1
	if nextFileCount > maxCopyDirsFileCount {
		slog.Warn("[WARN-GIT] aborting copy_dirs walk due to file count limit",
			"dir", dirEntry,
			"limit", maxCopyDirsFileCount,
			"processedFiles", budget.fileCount)
		*hadError = true
		return false, filepath.SkipAll
	}
	if budget.totalSize > maxCopyDirsTotalBytes || fileSize > maxCopyDirsTotalBytes-budget.totalSize {
		slog.Warn("[WARN-GIT] aborting copy_dirs walk due to total size limit",
			"dir", dirEntry,
			"limitBytes", maxCopyDirsTotalBytes,
			"processedBytes", budget.totalSize,
			"path", srcPath,
			"nextFileSize", fileSize)
		*hadError = true
		return false, filepath.SkipAll
	}
	nextTotalSize := budget.totalSize + fileSize
	budget.fileCount = nextFileCount
	budget.totalSize = nextTotalSize
	return true, nil
}

func copyFileByStreaming(srcPath, dstPath string) (retErr error) {
	srcFile, openSrcErr := os.Open(srcPath)
	if openSrcErr != nil {
		if errors.Is(openSrcErr, os.ErrNotExist) {
			return openSrcErr
		}
		return fmt.Errorf("open source file: %w", openSrcErr)
	}
	defer closeFileAndJoinError(srcFile, "source file", &retErr)

	// Create destination files with owner-only permissions.
	// We intentionally do not preserve source mode bits for copied config data.
	dstFile, openDstErr := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if openDstErr != nil {
		return fmt.Errorf("open destination file: %w", openDstErr)
	}
	synced := false
	defer func() {
		if retErr == nil || synced {
			return
		}
		if removeErr := removeFileFn(dstPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove partial destination file: %w", removeErr))
		}
	}()
	defer closeFileAndJoinError(dstFile, "destination file", &retErr)

	if _, copyErr := streamCopyFn(dstFile, srcFile); copyErr != nil {
		return fmt.Errorf("stream copy file: %w", copyErr)
	}
	if syncErr := syncFileFn(dstFile); syncErr != nil {
		return fmt.Errorf("sync destination file: %w", syncErr)
	}
	synced = true
	return nil
}

func closeFileAndJoinError(file *os.File, label string, retErr *error) {
	if file == nil {
		return
	}
	if closeErr := file.Close(); closeErr != nil {
		wrapped := fmt.Errorf("close %s: %w", label, closeErr)
		if *retErr == nil {
			*retErr = wrapped
			return
		}
		*retErr = errors.Join(*retErr, wrapped)
	}
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
// When opts.EnableAgentTeam is true, Agent Teams environment variables are set.
func (a *App) CreateSessionWithExistingWorktree(
	repoPath string,
	sessionName string,
	worktreePath string,
	opts CreateSessionOptions,
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
		if errors.Is(err, os.ErrNotExist) {
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

	createdName, err = a.createSessionForDirectory(router, worktreePath, sessionName, opts)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Set session-level env flags before any additional pane can be created.
	applySessionEnvFlags(sessions, createdName, opts.UseClaudeEnv, opts.UsePaneEnv)

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
