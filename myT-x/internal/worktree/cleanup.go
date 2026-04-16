package worktree

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	gitpkg "myT-x/internal/git"
)

// CleanupWorktree manually removes the worktree associated with a session.
func (s *Service) CleanupWorktree(sessionName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return err
	}
	worktreeInfo, err := s.requireWorktreeInfo(sessionName)
	if err != nil {
		return err
	}
	wtPath := worktreeInfo.Path
	repoPath := worktreeInfo.RepoPath
	cfg := s.deps.GetConfigSnapshot()

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	if !cfg.Worktree.ForceCleanup {
		if err := gitpkg.CheckWorktreeCleanForRemoval(wtPath); err != nil {
			return fmt.Errorf("failed to remove worktree safely: %w", err)
		}
	}

	if err := repo.RemoveWorktree(wtPath); err != nil {
		if !cfg.Worktree.ForceCleanup {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}
		slog.Warn("[WARN-GIT] normal worktree removal failed, trying forced removal",
			"session", sessionName, "path", wtPath, "error", err)
		if fErr := repo.RemoveWorktreeForced(wtPath); fErr != nil {
			return fmt.Errorf("failed to remove worktree (forced): %w", fErr)
		}
	}

	gitpkg.PostRemovalCleanup(repo, wtPath)

	s.deps.CleanupOrphanedLocalBranch(sessionName, repo, worktreeInfo.BranchName)

	// Clear worktree metadata.
	return sessions.SetWorktreeInfo(sessionName, nil)
}
