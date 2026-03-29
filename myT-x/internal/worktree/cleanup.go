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

	gitpkg.PostRemovalCleanup(repo, wtPath)

	s.deps.CleanupOrphanedLocalBranch(sessionName, repo, worktreeInfo.BranchName)

	// Clear worktree metadata.
	return sessions.SetWorktreeInfo(sessionName, nil)
}
