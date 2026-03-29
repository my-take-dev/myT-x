package worktree

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	gitpkg "myT-x/internal/git"
)

// OrphanedWorktree describes a worktree that is not associated with any active session.
type OrphanedWorktree struct {
	Path       string                 `json:"path"`
	BranchName string                 `json:"branchName"`
	HasChanges bool                   `json:"hasChanges"`
	Health     *gitpkg.WorktreeHealth `json:"health,omitempty"`
}

// normalizeWorktreePath returns a canonical form of a worktree path for
// reliable map lookups on Windows (case-insensitive, cleaned separators).
func normalizeWorktreePath(p string) string {
	return strings.ToLower(filepath.Clean(p))
}

// ListOrphanedWorktrees returns worktree directories under .wt/ that are
// not associated with any active session.
func (s *Service) ListOrphanedWorktrees(repoPath string) ([]OrphanedWorktree, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	cfg := s.deps.GetConfigSnapshot()
	if !cfg.Worktree.Enabled {
		return nil, nil
	}

	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return nil, err
	}

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	worktrees, err := repo.ListWorktreesWithInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Collect all worktree paths that are tied to active sessions.
	// Use normalized paths for reliable comparison on Windows.
	activeWtPaths := make(map[string]struct{})
	snapshot := sessions.Snapshot()
	for _, sess := range snapshot {
		if sess.Worktree != nil && sess.Worktree.IsWorktreeSession() {
			activeWtPaths[normalizeWorktreePath(sess.Worktree.Path)] = struct{}{}
		}
	}

	var orphans []OrphanedWorktree
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		if _, active := activeWtPaths[normalizeWorktreePath(wt.Path)]; active {
			continue
		}

		orphan := OrphanedWorktree{
			Path:       wt.Path,
			BranchName: wt.Branch,
		}

		// Attach health status.
		health := repo.CheckWorktreeHealth(wt.Path)
		orphan.Health = &health

		// Check for uncommitted changes (best-effort).
		wtRepo, openErr := gitpkg.Open(wt.Path)
		if openErr != nil {
			slog.Debug("[DEBUG-GIT] ListOrphanedWorktrees: cannot open worktree for change check",
				"path", wt.Path, "error", openErr)
			orphan.HasChanges = true // conservative: treat as dirty if we cannot check
		} else {
			hasChanges, chkErr := wtRepo.HasUncommittedChanges()
			if chkErr != nil {
				slog.Debug("[DEBUG-GIT] ListOrphanedWorktrees: change check failed",
					"path", wt.Path, "error", chkErr)
				orphan.HasChanges = true
			} else {
				orphan.HasChanges = hasChanges
			}
		}

		orphans = append(orphans, orphan)
	}

	return orphans, nil
}
