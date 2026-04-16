package worktree

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/tmux"
)

// CreateSessionWithWorktree creates a new session backed by a git worktree.
// The worktree is placed at {parentDir}/{repoName}.wt/{identifier}.
func (s *Service) CreateSessionWithWorktree(
	repoPath string,
	sessionName string,
	opts WorktreeSessionOptions,
) (snapshot tmux.SessionSnapshot, retErr error) {
	if s.deps.IsShuttingDown() {
		return tmux.SessionSnapshot{}, errors.New("cannot create worktree session: application is shutting down")
	}
	sessions, err := s.deps.RequireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	repoPath = strings.TrimSpace(repoPath)
	sessionName = strings.TrimSpace(sessionName)
	sessionName = tmux.SanitizeSessionName(sessionName, "worktree-session")
	opts.BranchName = strings.TrimSpace(opts.BranchName)
	opts.BaseBranch = strings.TrimSpace(opts.BaseBranch)
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("session name is required")
	}
	sessionName, releaseSessionName := s.reserveAvailableSessionName(sessionName)
	defer releaseSessionName()
	validatedBranchName, err := validateAndTrimWorktreeBranchName(opts.BranchName)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	opts.BranchName = validatedBranchName
	cfg := s.deps.GetConfigSnapshot()
	createdName := ""
	wtPath := ""
	worktreeCreated := false
	setupScriptsStopped := true
	var repo *gitpkg.Repository
	var setupScriptsCancel context.CancelFunc
	var setupScriptsDone chan struct{}
	setupTimeout := cfg.Worktree.SetupScriptTimeout()
	// NOTE: Emit snapshots on both success and rollback paths so frontend
	// subscribers stay synchronized even when RPC return values are not consumed.
	defer func() {
		if retErr == nil {
			return
		}
		if setupScriptsCancel != nil {
			setupScriptsCancel()
			if !waitForSetupScriptsCancellation(setupScriptsDone, config.SetupScriptCancellationWait) {
				setupScriptsStopped = false
				eventSessionName := strings.TrimSpace(createdName)
				if eventSessionName == "" {
					eventSessionName = strings.TrimSpace(sessionName)
				}
				s.deps.EmitWorktreeCleanupFailure(eventSessionName, wtPath,
					errors.New("setup scripts did not stop before rollback timeout"))
				slog.Warn("[WARN-GIT] timed out waiting for setup scripts to stop during rollback; "+
					"skipping worktree cleanup to avoid deleting the directory while scripts may still be running",
					"session", eventSessionName, "worktree", wtPath, "timeout", config.SetupScriptCancellationWait)
				retErr = fmt.Errorf("%w (setup scripts did not stop before rollback; worktree cleanup skipped)", retErr)
			}
		}
		if createdName != "" {
			if rollbackSessionErr := s.deps.RollbackCreatedSession(createdName); rollbackSessionErr != nil {
				retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackSessionErr)
			}
			// Notify frontend after session rollback for UI consistency (#69).
			s.deps.RequestSnapshot(true)
		}
		if worktreeCreated && repo != nil && wtPath != "" && setupScriptsStopped {
			if rollbackErr := rollbackWorktree(repo, wtPath, opts.BranchName); rollbackErr != nil {
				eventSessionName := strings.TrimSpace(createdName)
				if eventSessionName == "" {
					eventSessionName = strings.TrimSpace(sessionName)
				}
				s.deps.EmitWorktreeCleanupFailure(eventSessionName, wtPath, rollbackErr)
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

	repo, err = gitpkg.Open(repoPath)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to open repository: %w", err)
	}

	wtResult, err := createWorktreeForSession(repo, repoPath, sessionName, opts, s.deps.CurrentBranch)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	wtPath = wtResult.WtPath
	worktreeCreated = true

	if wtResult.PullFailed {
		s.deps.Emitter.Emit("worktree:pull-failed", map[string]any{
			"sessionName": sessionName,
			"message":     "pull failed, worktree created from local state",
			"error":       wtResult.PullError,
		})
	}

	createdName, err = s.deps.CreateSession(wtPath, sessionName, opts.EnableAgentTeam, opts.UseClaudeEnv, opts.UsePaneEnv)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Set session-level env flags before any additional pane can be created.
	s.deps.ApplySessionEnvFlags(sessions, createdName, opts.UseClaudeEnv, opts.UsePaneEnv, opts.UseSessionPaneScope)

	// Store worktree metadata on the session.
	if err := sessions.SetWorktreeInfo(createdName, &tmux.SessionWorktreeInfo{
		Path:       wtPath,
		RepoPath:   repoPath,
		BranchName: opts.BranchName,
		BaseBranch: wtResult.ResolvedBaseBranch,
		IsDetached: false,
	}); err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to set worktree info: %w", err)
	}

	if err := s.deps.StoreRootPath(createdName, repoPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Copy configured files (e.g. .env) from repo to worktree.
	if copyFailures := s.CopyConfigFilesToWorktree(repoPath, wtPath, cfg.Worktree.CopyFiles); len(copyFailures) > 0 {
		slog.Warn("[WARN-GIT] failed to copy one or more configured files to worktree",
			"session", createdName, "path", wtPath, "files", copyFailures)
		s.deps.Emitter.Emit("worktree:copy-files-failed", map[string]any{
			"sessionName": createdName,
			"files":       copyFailures,
		})
	}

	// Copy configured directories (e.g. .vscode) from repo to worktree.
	if copyDirFailures := s.CopyConfigDirsToWorktree(repoPath, wtPath, cfg.Worktree.CopyDirs); len(copyDirFailures) > 0 {
		slog.Warn("[WARN-GIT] failed to copy one or more configured directories to worktree",
			"session", createdName, "path", wtPath, "dirs", copyDirFailures)
		s.deps.Emitter.Emit("worktree:copy-dirs-failed", map[string]any{
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
		if appCtx := s.deps.RuntimeContext(); appCtx != nil {
			parentCtx = appCtx
		}
		setupScriptsCtx, cancel := context.WithCancel(parentCtx)
		setupScriptsCancel = cancel
		setupScriptsDone = make(chan struct{})
		releaseTrackedCancel := func() {}
		skipSetupWorkerDone := false
		shouldStartSetupWorker := true
		if s.deps.RegisterSetupWorker != nil {
			releaseTrackedCancel, shouldStartSetupWorker = s.deps.RegisterSetupWorker(cancel)
			skipSetupWorkerDone = true
		} else {
			s.deps.SetupWGAdd(1)
			if s.deps.TrackSetupCancel != nil {
				releaseTrackedCancel = s.deps.TrackSetupCancel(cancel)
			}
		}
		if !shouldStartSetupWorker {
			close(setupScriptsDone)
		} else {
			go func(ctx context.Context, cancel context.CancelFunc, done chan struct{}, release func(), skipDone bool) {
				defer close(done)
				if !skipDone {
					defer s.deps.SetupWGDone()
				}
				defer release()
				defer cancel()
				defer func() {
					s.deps.RecoverBackgroundPanic("worktree-setup-scripts", recover())
				}()
				s.runSetupScriptsWithTimeout(ctx, wtPath, createdName, cfg.Shell, cfg.Worktree.SetupScripts, setupTimeout)
			}(setupScriptsCtx, cancel, setupScriptsDone, releaseTrackedCancel, skipSetupWorkerDone)
		}
	}

	snapshot, retErr = s.deps.ActivateCreatedSession(createdName)
	if retErr == nil {
		s.deps.RequestSnapshot(true)
	}
	return snapshot, retErr
}

// CreateSessionWithExistingWorktree creates a session using an existing worktree.
// No new worktree is created; the session opens in the given worktree path.
// Returns an error if the worktree path is already in use by another session.
func (s *Service) CreateSessionWithExistingWorktree(
	repoPath string,
	sessionName string,
	worktreePath string,
	opts SessionEnvOptions,
) (snapshot tmux.SessionSnapshot, retErr error) {
	if s.deps.IsShuttingDown() {
		return tmux.SessionSnapshot{}, errors.New("cannot create worktree session: application is shutting down")
	}
	sessions, err := s.deps.RequireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	repoPath = strings.TrimSpace(repoPath)
	sessionName = strings.TrimSpace(sessionName)
	sessionName = tmux.SanitizeSessionName(sessionName, "worktree-session")
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
	sessionName, releaseSessionName := s.reserveAvailableSessionName(sessionName)
	defer releaseSessionName()
	cfg := s.deps.GetConfigSnapshot()

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
	if conflict := s.deps.FindSessionByWorktreePath(worktreePath); conflict != "" {
		return tmux.SessionSnapshot{}, fmt.Errorf(
			"worktree path is already in use by session %q: %s", conflict, worktreePath)
	}

	// Detect current branch of the existing worktree.
	var branchName string
	var isDetached bool
	wtRepo, err := gitpkg.Open(worktreePath)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to open worktree: %w", err)
	}
	isDetached, err = wtRepo.IsDetachedHead()
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to check HEAD state: %w", err)
	}
	if isDetached {
		branchName = ""
	} else {
		branchName, err = s.deps.CurrentBranch(wtRepo)
		if err != nil {
			return tmux.SessionSnapshot{}, fmt.Errorf("failed to detect current branch: %w", err)
		}
	}

	createdName := ""
	defer func() {
		if retErr == nil || createdName == "" {
			return
		}
		if rollbackErr := s.deps.RollbackCreatedSession(createdName); rollbackErr != nil {
			slog.Warn("[WARN-GIT] rollback kill-session failed", "session", createdName, "error", rollbackErr)
			retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackErr)
		}
		// Notify frontend after session rollback for UI consistency (#69).
		s.deps.RequestSnapshot(true)
	}()

	createdName, err = s.deps.CreateSession(worktreePath, sessionName, opts.EnableAgentTeam, opts.UseClaudeEnv, opts.UsePaneEnv)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	// Set session-level env flags before any additional pane can be created.
	s.deps.ApplySessionEnvFlags(sessions, createdName, opts.UseClaudeEnv, opts.UsePaneEnv, opts.UseSessionPaneScope)

	if err := sessions.SetWorktreeInfo(createdName, &tmux.SessionWorktreeInfo{
		Path:       worktreePath,
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseBranch: "",
		IsDetached: isDetached,
	}); err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("failed to set worktree info: %w", err)
	}

	if err := s.deps.StoreRootPath(createdName, repoPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}
	snapshot, retErr = s.deps.ActivateCreatedSession(createdName)
	if retErr == nil {
		s.deps.RequestSnapshot(true)
	}
	return snapshot, retErr
}

// runSetupScriptsWithParentContext runs setup scripts sequentially with the
// default per-script timeout. Tests call this helper directly.
func (s *Service) runSetupScriptsWithParentContext(parentCtx context.Context, wtPath, sessionName, shell string, scripts []string) {
	s.runSetupScriptsWithTimeout(parentCtx, wtPath, sessionName, shell, scripts,
		config.WorktreeConfig{}.SetupScriptTimeout())
}

// runSetupScriptsWithTimeout runs setup scripts sequentially with a per-script
// timeout. Called asynchronously from CreateSessionWithWorktree.
func (s *Service) runSetupScriptsWithTimeout(
	parentCtx context.Context,
	wtPath, sessionName, shell string,
	scripts []string,
	setupTimeout time.Duration,
) {
	if setupTimeout <= 0 {
		setupTimeout = config.WorktreeConfig{}.SetupScriptTimeout()
	}
	if strings.TrimSpace(shell) == "" {
		shell = "powershell.exe"
	}

	// If parent context is not provided, use app context so scripts are cancelled
	// on app shutdown. When app context is nil (startup race), fall back to
	// Background; each script still has setupTimeout so it will not run forever.
	if parentCtx == nil {
		parentCtx = s.deps.RuntimeContext()
		if parentCtx == nil {
			parentCtx = context.Background()
			slog.Warn("[WARN-GIT] runSetupScripts: app context not yet available, using background context",
				"session", sessionName)
		}
	}
	// Emit using the latest app context when available; otherwise fall back to
	// the parent context used by script execution.
	latestAppCtx := func() context.Context {
		if current := s.deps.RuntimeContext(); current != nil {
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
		output, err := s.deps.ExecuteSetupCommand(ctx, shell, shellFlag, script, wtPath)
		cancel()

		if err != nil {
			slog.Warn("[WARN-GIT] setup script failed",
				"session", sessionName, "script", script,
				"error", err, "output", string(output))
			s.deps.Emitter.EmitWithContext(latestAppCtx(), "worktree:setup-complete", map[string]any{
				"sessionName": sessionName,
				"success":     false,
				"error":       fmt.Sprintf("script %q failed: %v", script, err),
			})
			return
		}

		slog.Debug("[DEBUG-GIT] setup script completed",
			"session", sessionName, "script", script)
	}

	s.deps.Emitter.EmitWithContext(latestAppCtx(), "worktree:setup-complete", map[string]any{
		"sessionName": sessionName,
		"success":     true,
	})
}
