package main

// FileEntry represents a single file or directory in a directory listing.
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // root-relative path
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"` // file size in bytes (0 for directories)
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
	Branch    string   `json:"branch"`
	Modified  []string `json:"modified"`
	Staged    []string `json:"staged"`
	Untracked []string `json:"untracked"`
	Ahead     int      `json:"ahead"`
	Behind    int      `json:"behind"`
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
