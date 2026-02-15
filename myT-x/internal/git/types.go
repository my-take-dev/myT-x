package git

// WorktreeInfo represents information about a git worktree.
type WorktreeInfo struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	IsMain     bool   `json:"isMain"`
	IsDetached bool   `json:"isDetached"`
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
