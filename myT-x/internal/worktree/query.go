package worktree

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	gitpkg "myT-x/internal/git"
)

// CheckWorktreeStatus returns the worktree status for a session.
// Used by the frontend to decide what confirmation dialog to show before closing.
func (s *Service) CheckWorktreeStatus(sessionName string) (WorktreeStatus, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return WorktreeStatus{}, errors.New("session name is required")
	}
	sessions, err := s.deps.RequireSessions()
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

// ListWorktreesByRepo returns all worktree information for a given repository.
// Stale entries (folders that no longer exist) are pruned before listing
// so the UI never shows invalid worktrees.
// Each non-main worktree includes a health status check.
//
// NOTE: Health checks run O(N) git commands per worktree. This is acceptable
// for typical worktree counts (< 10). If performance becomes an issue with
// many worktrees, consider adding an option to skip health checks.
func (s *Service) ListWorktreesByRepo(repoPath string) ([]gitpkg.WorktreeInfo, error) {
	repo, err := gitpkg.Open(strings.TrimSpace(repoPath))
	if err != nil {
		return nil, err
	}
	// NOTE: Prune failure is non-fatal; proceed with listing even if prune
	// fails so the user still sees available worktrees.
	if pruneErr := repo.PruneWorktrees(); pruneErr != nil {
		slog.Warn("[WARN-GIT] failed to prune worktrees before listing", "error", pruneErr)
	}
	worktrees, err := repo.ListWorktreesWithInfo()
	if err != nil {
		return nil, err
	}

	// Attach health status to non-main worktrees.
	for i := range worktrees {
		if worktrees[i].IsMain {
			continue
		}
		health := repo.CheckWorktreeHealth(worktrees[i].Path)
		worktrees[i].Health = &health
	}
	return worktrees, nil
}

// ListBranches returns all local branch names for the repository at the given path.
func (s *Service) ListBranches(repoPath string) ([]string, error) {
	repo, err := gitpkg.Open(strings.TrimSpace(repoPath))
	if err != nil {
		return nil, err
	}
	// NOTE: PruneWorktrees is not needed here because ListBranchesForWorktreeBase
	// reads git refs directly and does not depend on worktree metadata.
	return repo.ListBranchesForWorktreeBase()
}

// GetCurrentBranch returns the current branch of the repository at repoPath.
// Returns "" for detached HEAD state.
func (s *Service) GetCurrentBranch(repoPath string) (string, error) {
	repo, err := gitpkg.Open(strings.TrimSpace(repoPath))
	if err != nil {
		return "", err
	}
	return repo.CurrentBranch()
}

// IsGitRepository checks if the given path is a git repository.
func (s *Service) IsGitRepository(path string) bool {
	return gitpkg.IsGitRepository(strings.TrimSpace(path))
}

// CheckWorktreePathConflict checks whether the given worktree path is already
// used by an active session. Returns the session name if conflict exists, or "".
func (s *Service) CheckWorktreePathConflict(worktreePath string) string {
	return s.deps.FindSessionByWorktreePath(strings.TrimSpace(worktreePath))
}
