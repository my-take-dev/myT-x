package worktree

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/tmux"
)

// requireWorktreeInfo returns metadata for sessions that are backed by an
// actual worktree directory (Path must be non-empty).
// Repo-tracked sessions that only carry RepoPath/BranchName are rejected.
func (s *Service) requireWorktreeInfo(sessionName string) (*tmux.SessionWorktreeInfo, error) {
	sessions, err := s.deps.RequireSessions()
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

// validateAndTrimWorktreeBranchName trims and validates the branch name.
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

// chooseWorktreeIdentifier picks an identifier from branch or session name.
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

// createWorktreeResult holds the result of worktree creation.
type createWorktreeResult struct {
	WtPath             string
	ResolvedBaseBranch string
	PullFailed         bool
	PullError          string
}

// createWorktreeForSession creates the git worktree for a new session.
// Handles pull, path generation, validation, and the actual worktree creation.
// Pull failures are fatal by default. When ContinueOnPullFailure is enabled,
// the worktree is created from local state and PullFailed is set in the result
// for caller notification.
func createWorktreeForSession(
	repo *gitpkg.Repository, repoPath, sessionName string, opts WorktreeSessionOptions,
	currentBranch func(*gitpkg.Repository) (string, error),
) (result createWorktreeResult, err error) {
	if currentBranch == nil {
		currentBranch = func(repo *gitpkg.Repository) (string, error) {
			return repo.CurrentBranch()
		}
	}

	// BranchName is validated once in CreateSessionWithWorktree before this helper is called.
	branchName := opts.BranchName
	if branchName == "" {
		return createWorktreeResult{}, errors.New("branch name is required for new worktree creation")
	}

	if opts.PullBeforeCreate {
		if pullErr := repo.Pull(); pullErr != nil {
			if !opts.ContinueOnPullFailure {
				return createWorktreeResult{}, fmt.Errorf("pull before worktree creation failed: %w", pullErr)
			}
			slog.Warn("[WARN-GIT] pull before worktree creation failed, continuing with local state",
				"error", pullErr, "repoPath", repoPath)
			result.PullFailed = true
			result.PullError = pullErr.Error()
		}
	}

	identifier := chooseWorktreeIdentifier(branchName, sessionName)

	result.WtPath = gitpkg.FindAvailableWorktreePath(gitpkg.GenerateWorktreePath(repoPath, identifier))

	if err := gitpkg.ValidateWorktreePath(result.WtPath); err != nil {
		return createWorktreeResult{}, fmt.Errorf("invalid worktree path: %w", err)
	}

	wtDir := gitpkg.GenerateWorktreeDirPath(repoPath)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return createWorktreeResult{}, fmt.Errorf("failed to create worktree directory %s: %w", wtDir, err)
	}

	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		isDetached, headErr := repo.IsDetachedHead()
		if headErr != nil {
			return createWorktreeResult{}, fmt.Errorf("failed to check HEAD state: %w", headErr)
		}
		if isDetached {
			baseBranch = "HEAD"
		} else {
			resolved, brErr := currentBranch(repo)
			if brErr != nil {
				return createWorktreeResult{}, fmt.Errorf("failed to detect current branch: %w", brErr)
			}
			resolved = strings.TrimSpace(resolved)
			if resolved == "" {
				return createWorktreeResult{}, errors.New("failed to detect current branch: current branch is empty")
			}
			baseBranch = resolved
		}
	}

	if err := repo.CreateWorktree(result.WtPath, branchName, baseBranch); err != nil {
		return createWorktreeResult{}, fmt.Errorf("failed to create worktree: %w", err)
	}
	result.ResolvedBaseBranch = baseBranch

	slog.Debug("[DEBUG-GIT] worktree created",
		"path", result.WtPath, "repoPath", repoPath, "detached", false)

	return result, nil
}

// rollbackWorktree removes a worktree and prunes orphaned entries.
// Returns the removal error (if any) for inclusion in the caller's error message.
func rollbackWorktree(repo *gitpkg.Repository, wtPath, branchName string) error {
	var rollbackErr error
	if rmErr := repo.RemoveWorktreeForced(wtPath); rmErr != nil {
		slog.Warn("[WARN-GIT] failed to rollback worktree", "error", rmErr)
		rollbackErr = fmt.Errorf("failed to remove worktree during rollback: %w", rmErr)
	}
	gitpkg.PostRemovalCleanup(repo, wtPath)
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

// waitForSetupScriptsCancellation waits for the setup scripts goroutine to
// finish, with a timeout. Returns true if the goroutine finished in time.
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

// shellExecFlag returns the command-execution flag for the given shell.
func shellExecFlag(shell string) string {
	base := config.CanonicalShellBaseName(shell)
	switch base {
	case "cmd.exe":
		return "/c"
	case "bash.exe", "wsl.exe":
		return "-c"
	case "powershell.exe", "pwsh.exe":
		return "-Command"
	default:
		slog.Warn("[WARN-GIT] unrecognized setup shell; defaulting to PowerShell command flag",
			"shell", shell, "base", strings.ToLower(filepath.Base(shell)))
		return "-Command"
	}
}

// rollbackPromotedWorktreeBranch restores detached HEAD and deletes the branch
// that was created during a failed promotion.
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
