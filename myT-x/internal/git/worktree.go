package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CreateWorktree creates a new worktree with a new branch from the specified base branch.
// Executes: git worktree add -b <new-branch> -- <path> <commit-ish>
func (r *Repository) CreateWorktree(worktreePath, branchName, baseBranch string) error {
	if err := ValidateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}
	_, err := r.runGitCommand("worktree", "add", "-b", branchName, "--", worktreePath, baseBranch)
	return err
}

// CreateWorktreeFromBranch creates a worktree from an existing branch without creating a new branch.
// Executes: git worktree add -- <path> <existing-branch>
func (r *Repository) CreateWorktreeFromBranch(worktreePath, existingBranch string) error {
	if err := ValidateBranchName(existingBranch); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}
	_, err := r.runGitCommand("worktree", "add", "--", worktreePath, existingBranch)
	return err
}

// CreateWorktreeDetached creates a worktree in detached HEAD state.
// Executes: git worktree add --detach -- <path> [<commit-ish>]
func (r *Repository) CreateWorktreeDetached(worktreePath, commitish string) error {
	args := []string{"worktree", "add", "--detach", "--", worktreePath}
	if commitish != "" {
		args = append(args, commitish)
	}
	_, err := r.runGitCommand(args...)
	return err
}

// RemoveWorktree removes a worktree.
// Executes: git worktree remove -- <path>
func (r *Repository) RemoveWorktree(worktreePath string) error {
	_, err := r.runGitCommand("worktree", "remove", "--", worktreePath)
	return err
}

// RemoveWorktreeForced removes a worktree even with uncommitted changes.
// Executes: git worktree remove --force -- <path>
func (r *Repository) RemoveWorktreeForced(worktreePath string) error {
	_, err := r.runGitCommand("worktree", "remove", "--force", "--", worktreePath)
	return err
}

// ListWorktrees returns a list of worktree paths.
func (r *Repository) ListWorktrees() ([]string, error) {
	output, err := r.runGitCommand("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			// git returns forward slashes on Windows; normalize to OS path separator.
			worktrees = append(worktrees, filepath.FromSlash(strings.TrimPrefix(line, "worktree ")))
		}
	}
	return worktrees, nil
}

// ListWorktreesWithInfo returns detailed information about all worktrees.
func (r *Repository) ListWorktreesWithInfo() ([]WorktreeInfo, error) {
	output, err := r.runGitCommand("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	lines := strings.Split(output, "\n")

	var current WorktreeInfo
	isFirst := true
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if !isFirst && current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{
				// git returns forward slashes on Windows; normalize to OS path separator.
				Path:   filepath.FromSlash(strings.TrimPrefix(line, "worktree ")),
				IsMain: isFirst,
			}
			isFirst = false
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "detached":
			current.IsDetached = true
		case line == "bare":
			current.Path = ""
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// PruneWorktrees removes stale worktree entries (broken links) immediately.
func (r *Repository) PruneWorktrees() error {
	_, err := r.runGitCommand("worktree", "prune", "--expire=now")
	return err
}
