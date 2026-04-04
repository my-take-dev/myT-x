package devpanel

// FileEntry represents a single file or directory in a directory listing.
type FileEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"` // root-relative path
	IsDir       bool   `json:"is_dir"`
	Size        int64  `json:"size"` // file size in bytes (0 for directories)
	HasChildren bool   `json:"has_children,omitempty"`
}

// FileContent represents the contents of a file read from disk.
type FileContent struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	LineCount int    `json:"line_count"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"` // true if file exceeded 1MB limit
	Binary    bool   `json:"binary"`    // true if binary content detected
}

// FileMetadata represents stat information for a file-system entry.
type FileMetadata struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

// WriteFileResult represents the outcome of a write/create operation.
type WriteFileResult struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// GitGraphCommit represents a single commit for graph rendering.
type GitGraphCommit struct {
	Hash       string   `json:"hash"` // short hash (7 chars)
	FullHash   string   `json:"full_hash"`
	Parents    []string `json:"parents"` // parent full hashes (for graph drawing)
	Subject    string   `json:"subject"`
	AuthorName string   `json:"author_name"`
	AuthorDate string   `json:"author_date"` // ISO 8601
	Refs       []string `json:"refs"`        // branch/tag names
}

// GitStatusResult represents the working tree status of a git repository.
type GitStatusResult struct {
	Branch     string   `json:"branch"`
	Modified   []string `json:"modified"`
	Staged     []string `json:"staged"`
	Untracked  []string `json:"untracked"`
	Conflicted []string `json:"conflicted"`
	// Ahead is the number of commits ahead of the upstream tracking branch.
	// Meaningful only when UpstreamConfigured is true; when false, the value
	// is 0 and represents "data not available", not "no difference".
	Ahead int `json:"ahead"`
	// Behind is the number of commits behind the upstream tracking branch.
	// Meaningful only when UpstreamConfigured is true; when false, the value
	// is 0 and represents "data not available", not "no difference".
	Behind int `json:"behind"`
	// UpstreamConfigured is true when the current branch has a resolvable
	// upstream tracking branch AND the ahead/behind counts were successfully
	// parsed from git rev-list output.
	UpstreamConfigured bool `json:"upstream_configured"`
}

// WorkingDiffStatus represents the change status of a file.
//
// NOTE: This is a type alias (= string), not a defined type. Using a type alias
// allows the Wails TypeScript binding generator to emit the fields as plain
// string literals in the generated .d.ts, enabling direct use of string constants
// on the frontend without additional type casting.
// Trade-off: type safety at compile time is sacrificed.
type WorkingDiffStatus = string

const (
	WorkingDiffStatusModified  WorkingDiffStatus = "modified"
	WorkingDiffStatusAdded     WorkingDiffStatus = "added"
	WorkingDiffStatusDeleted   WorkingDiffStatus = "deleted"
	WorkingDiffStatusRenamed   WorkingDiffStatus = "renamed"
	WorkingDiffStatusUntracked WorkingDiffStatus = "untracked"
)

// WorkingDiffFile represents a single file's diff from git diff HEAD.
type WorkingDiffFile struct {
	Path      string            `json:"path"`
	OldPath   string            `json:"old_path"`
	Status    WorkingDiffStatus `json:"status"` // "modified" | "added" | "deleted" | "renamed" | "untracked"
	Additions int               `json:"additions"`
	Deletions int               `json:"deletions"`
	Diff      string            `json:"diff"`
}

// WorkingDiffResult represents the aggregated working diff for a session.
type WorkingDiffResult struct {
	Files        []WorkingDiffFile `json:"files"`
	TotalAdded   int               `json:"total_added"`
	TotalDeleted int               `json:"total_deleted"`
	Truncated    bool              `json:"truncated"`
}

// CommitResult represents the result of a git commit operation.
type CommitResult struct {
	Hash    string `json:"hash"`    // short commit hash
	Message string `json:"message"` // first line of commit message
}

// PushResult represents the result of a git push operation.
type PushResult struct {
	RemoteName  string `json:"remote_name"`  // e.g. "origin"
	BranchName  string `json:"branch_name"`  // e.g. "main"
	UpstreamSet bool   `json:"upstream_set"` // true if --set-upstream was used
}

// PullResult represents the result of a git pull operation.
type PullResult struct {
	Updated bool   `json:"updated"` // true if any changes were pulled
	Summary string `json:"summary"` // human-readable summary
}

// SearchFileResult represents a file match from file search.
type SearchFileResult struct {
	Path         string              `json:"path"`          // root-relative forward-slash path
	Name         string              `json:"name"`          // filename (last segment)
	IsNameMatch  bool                `json:"is_name_match"` // true if filename matched query
	ContentLines []SearchContentLine `json:"content_lines"` // matching content lines (empty for name-only matches)
}

// SearchContentLine represents a single matching line within a file.
type SearchContentLine struct {
	Line    int    `json:"line"`    // 1-based line number
	Content string `json:"content"` // line text (truncated to 500 chars)
}
