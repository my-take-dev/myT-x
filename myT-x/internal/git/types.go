package git

// WorktreeInfo represents information about a git worktree.
type WorktreeInfo struct {
	Path       string          `json:"path"`
	Branch     string          `json:"branch"`
	IsMain     bool            `json:"isMain"`
	IsDetached bool            `json:"isDetached"`
	Health     *WorktreeHealth `json:"health,omitempty"`
}

// WorktreeHealth represents the health status of a worktree directory.
// Invariant: IsHealthy == (len(Issues) == 0). Both fields are maintained by
// CheckWorktreeHealth; callers should not construct WorktreeHealth directly.
type WorktreeHealth struct {
	IsHealthy bool     `json:"isHealthy"`
	Issues    []string `json:"issues,omitempty"`
}

// Repository wraps git CLI operations.
// All operations use system git CLI (no embedded git library).
type Repository struct {
	path string
}

// GetPath returns the repository root path.
func (r *Repository) GetPath() string {
	return r.path
}
