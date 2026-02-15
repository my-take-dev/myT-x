package git

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Open opens an existing git repository using CLI-only detection.
func Open(path string) (*Repository, error) {
	start := time.Now()
	defer func() {
		slog.Debug("[DEBUG-GIT] Open repository",
			"duration_ms", time.Since(start).Milliseconds(),
			"path", path)
	}()

	_, err := executeGitCommandAt(path, []string{"rev-parse", "--git-dir"})
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s: %w", path, err)
	}
	return &Repository{path: path}, nil
}

// IsGitRepository checks if the path is a git repository.
// Uses executeGitCommandAt to respect the semaphore concurrency limit.
func IsGitRepository(path string) bool {
	start := time.Now()
	_, err := executeGitCommandAt(path, []string{"rev-parse", "--git-dir"})
	slog.Debug("[DEBUG-GIT] IsGitRepository check",
		"duration_ms", time.Since(start).Milliseconds(),
		"path", path,
		"isGitRepo", err == nil)
	return err == nil
}

// FindRepoRoot returns the root directory of the git repository.
// Returns ("", error) if path is not inside a git repository.
func FindRepoRoot(path string) (string, error) {
	output, err := executeGitCommandAt(path, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", fmt.Errorf("failed to find repo root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CurrentBranch returns the name of the current branch, or empty string if detached HEAD.
func (r *Repository) CurrentBranch() (string, error) {
	output, err := r.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if output == "HEAD" {
		return "", nil // detached HEAD
	}
	return output, nil
}

// ListBranches returns all local branch names.
func (r *Repository) ListBranches() ([]string, error) {
	output, err := r.runGitCommand("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if output == "" {
		return []string{}, nil
	}
	lines := strings.Split(output, "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

type branchTrackingInfo struct {
	Name          string
	Upstream      string
	UpstreamTrack string
}

func (info branchTrackingInfo) hasLiveUpstream() bool {
	if info.Upstream == "" {
		return false
	}
	return !strings.Contains(strings.ToLower(info.UpstreamTrack), "[gone]")
}

// ListBranchesForWorktreeBase returns branches suitable for "base branch" selection.
// Branches that only exist locally (e.g., worktree-only ephemeral branches) are excluded
// when remote-tracking metadata is available.
func (r *Repository) ListBranchesForWorktreeBase() ([]string, error) {
	infos, err := r.listLocalBranchTrackingInfo()
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return []string{}, nil
	}

	remoteBranchNames, err := r.listRemoteBranchNames()
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.Name == "" {
			continue
		}
		if info.hasLiveUpstream() {
			filtered = append(filtered, info.Name)
			continue
		}
		if _, exists := remoteBranchNames[info.Name]; exists {
			filtered = append(filtered, info.Name)
		}
	}

	// Fully local repositories may not have remote metadata. In that case we keep
	// local branches visible to avoid breaking branch selection UX.
	if len(filtered) == 0 && len(remoteBranchNames) == 0 && !hasAnyUpstreamTracking(infos) {
		return branchNamesFromTrackingInfo(infos), nil
	}
	return filtered, nil
}

// CleanupLocalBranchIfOrphaned removes a local branch when it has no corresponding
// remote branch and no live upstream tracking branch.
// Returns true when the local branch was deleted.
func (r *Repository) CleanupLocalBranchIfOrphaned(branchName string) (bool, error) {
	if err := ValidateBranchName(branchName); err != nil {
		return false, err
	}

	infos, err := r.listLocalBranchTrackingInfo()
	if err != nil {
		return false, err
	}

	var target branchTrackingInfo
	found := false
	for _, info := range infos {
		if info.Name != branchName {
			continue
		}
		target = info
		found = true
		break
	}
	if !found {
		return false, nil
	}
	if target.hasLiveUpstream() {
		return false, nil
	}

	remoteBranchNames, err := r.listRemoteBranchNames()
	if err != nil {
		return false, err
	}
	if _, exists := remoteBranchNames[branchName]; exists {
		return false, nil
	}

	if _, err := r.runGitCommand("branch", "-d", branchName); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) listLocalBranchTrackingInfo() ([]branchTrackingInfo, error) {
	output, err := r.runGitCommandRaw(
		"for-each-ref",
		"--format=%(refname:short)\t%(upstream:short)\t%(upstream:track)",
		"refs/heads",
	)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == "" {
		return []branchTrackingInfo{}, nil
	}

	lines := strings.Split(output, "\n")
	infos := make([]branchTrackingInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		info := branchTrackingInfo{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			info.Upstream = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			info.UpstreamTrack = strings.TrimSpace(parts[2])
		}
		if info.Name != "" {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

func (r *Repository) listRemoteBranchNames() (map[string]struct{}, error) {
	output, err := r.runGitCommand("for-each-ref", "--format=%(refname:short)", "refs/remotes")
	if err != nil {
		return nil, err
	}

	names := map[string]struct{}{}
	if strings.TrimSpace(output) == "" {
		return names, nil
	}

	for _, line := range strings.Split(output, "\n") {
		refName := strings.TrimSpace(line)
		if refName == "" {
			continue
		}
		parts := strings.SplitN(refName, "/", 2)
		if len(parts) != 2 {
			continue
		}
		branchName := strings.TrimSpace(parts[1])
		if branchName == "" || branchName == "HEAD" {
			continue
		}
		names[branchName] = struct{}{}
	}
	return names, nil
}

func hasAnyUpstreamTracking(infos []branchTrackingInfo) bool {
	for _, info := range infos {
		if strings.TrimSpace(info.Upstream) != "" {
			return true
		}
	}
	return false
}

func branchNamesFromTrackingInfo(infos []branchTrackingInfo) []string {
	branches := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.Name != "" {
			branches = append(branches, info.Name)
		}
	}
	return branches
}

// CheckoutNewBranch creates a new branch at the current HEAD and switches to it.
// This is used to promote a detached HEAD worktree to a named branch.
func (r *Repository) CheckoutNewBranch(branchName string) error {
	if err := ValidateBranchName(branchName); err != nil {
		return err
	}
	_, err := r.runGitCommand("checkout", "-b", branchName)
	return err
}

// HasUncommittedChanges checks if the worktree has uncommitted changes.
func (r *Repository) HasUncommittedChanges() (bool, error) {
	output, err := r.runGitCommand("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

// Pull fetches and fast-forward merges the current branch from origin.
func (r *Repository) Pull() error {
	_, err := r.runGitCommand("pull", "--ff-only")
	return err
}

// CommitAll stages all changes and commits with the given message.
func (r *Repository) CommitAll(message string) error {
	if message == "" {
		return fmt.Errorf("commit message must not be empty")
	}
	if _, err := r.runGitCommand("add", "-A"); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	if _, err := r.runGitCommand("commit", "-m", message); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	return nil
}

// Push pushes the current branch to origin.
func (r *Repository) Push() error {
	_, err := r.runGitCommand("push", "origin", "HEAD")
	return err
}

// HasUnpushedCommits checks if local has commits not yet pushed to upstream.
// Returns false if there is no upstream tracking branch (e.g. detached HEAD).
func (r *Repository) HasUnpushedCommits() (bool, error) {
	output, err := r.runGitCommand("log", "@{upstream}..HEAD", "--oneline")
	if err != nil {
		// Upstream-related errors: no upstream configured or no remote tracking.
		// Known error message patterns (git version/locale dependent):
		//   - "fatal: no upstream configured for branch 'xxx'" (git 2.x+)
		//   - "fatal: '@{upstream}' is not a valid ref" (detached HEAD)
		//   - "fatal: no such ref: 'xxx@{u}'" (missing upstream ref)
		//   - "fatal: HEAD does not point to a branch" (detached HEAD)
		// This intentionally avoids matching generic "not a valid ref" without
		// upstream tokens to prevent masking unrelated repository errors.
		// NOTE: Future enhancement — extract exit code from *exec.ExitError
		// to reduce dependence on locale-specific error message strings.
		if isNoUpstreamTrackingError(err.Error()) {
			slog.Debug("[DEBUG-GIT] HasUnpushedCommits: no upstream tracking branch",
				"path", r.path, "error", err)
			return false, nil
		}
		// Other errors (disk I/O, permission, etc.) are propagated.
		return false, fmt.Errorf("HasUnpushedCommits: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

func isNoUpstreamTrackingError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	if strings.Contains(lower, "no upstream configured") {
		return true
	}
	if strings.Contains(lower, "does not point to a branch") {
		return true
	}
	if strings.Contains(lower, "no such ref") && containsUpstreamToken(lower) {
		return true
	}
	if strings.Contains(lower, "not a valid ref") && containsUpstreamToken(lower) {
		return true
	}
	return false
}

func containsUpstreamToken(errMsg string) bool {
	return strings.Contains(errMsg, "@{u}") || strings.Contains(errMsg, "@{upstream}") || strings.Contains(errMsg, "upstream")
}
