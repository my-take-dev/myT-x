package worktree

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	gitpkg "myT-x/internal/git"
	"myT-x/internal/tmux"
)

// CommitAndPushWorktree commits and/or pushes changes in the session's worktree.
func (s *Service) CommitAndPushWorktree(sessionName, commitMessage string, push bool) error {
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
	if _, err := s.deps.RequireSessions(); err != nil {
		return err
	}

	worktreeInfo, err := s.requireWorktreeInfo(sessionName)
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

// PromoteWorktreeToBranch promotes a detached HEAD worktree to a named branch.
func (s *Service) PromoteWorktreeToBranch(sessionName string, branchName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	branchName = strings.TrimSpace(branchName)

	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return err
	}

	if err := gitpkg.ValidateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}

	worktreeInfo, err := s.requireWorktreeInfo(sessionName)
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

	s.deps.RequestSnapshot(true)
	return nil
}
