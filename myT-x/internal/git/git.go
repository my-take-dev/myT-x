package git

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Open opens an existing git repository using CLI-only detection.
func Open(path string) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}
	path = filepath.Clean(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	path = absPath

	start := time.Now()
	defer func() {
		slog.Debug("[DEBUG-GIT] Open repository",
			"duration_ms", time.Since(start).Milliseconds(),
			"path", path)
	}()

	_, err = runGitCLI(path, []string{"rev-parse", "--git-dir"})
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s: %w", path, err)
	}
	return &Repository{path: path}, nil
}

// IsGitRepository checks if the path is a git repository.
// Uses runGitCLI to respect the semaphore concurrency limit.
func IsGitRepository(path string) bool {
	start := time.Now()
	_, err := runGitCLI(path, []string{"rev-parse", "--git-dir"})
	slog.Debug("[DEBUG-GIT] IsGitRepository check",
		"duration_ms", time.Since(start).Milliseconds(),
		"path", path,
		"isGitRepo", err == nil)
	return err == nil
}

// FindRepoRoot returns the root directory of the git repository.
// Returns ("", error) if path is not inside a git repository.
func FindRepoRoot(path string) (string, error) {
	output, err := runGitCLI(path, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", fmt.Errorf("failed to find repo root: %w", err)
	}
	return filepath.FromSlash(strings.TrimSpace(string(output))), nil
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

// IsDetachedHead returns true if the repository is in detached HEAD state.
// Uses "git symbolic-ref" which fails (exit code 1) when HEAD is not a symbolic
// reference, indicating detached HEAD. A follow-up "git rev-parse HEAD" verifies
// that the repository is functional, distinguishing detached HEAD from real errors
// (disk I/O, permission denied, corrupt repository).
func (r *Repository) IsDetachedHead() (bool, error) {
	_, err := r.runGitCommand("symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// symbolic-ref fails for both detached HEAD and real errors.
		// Verify the repository is functional by checking if HEAD resolves.
		if _, verifyErr := r.runGitCommand("rev-parse", "HEAD"); verifyErr != nil {
			return false, fmt.Errorf("failed to determine HEAD state: %w", err)
		}
		// HEAD resolves but is not a symbolic ref → detached HEAD.
		return true, nil
	}
	return false, nil
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
	if len(filtered) == 0 && len(remoteBranchNames) == 0 && !hasAnyLiveUpstream(infos) {
		return branchNamesFromTrackingInfo(infos), nil
	}
	// When remotes exist, return only remote-backed/live-upstream branches.
	// This intentionally returns an empty list when every local branch is stale
	// or local-only, keeping ephemeral worktree branches hidden from selection.
	return filtered, nil
}

// CleanupLocalBranchIfOrphaned removes a local branch when it has no corresponding
// remote branch and no live upstream tracking branch.
// In fully local repositories (without remotes), all local branches are treated as
// orphan candidates by this method.
// Caller note: if remote-tracking refs may be stale, run "git fetch --prune"
// before calling this method to avoid preserving branches based on outdated refs.
// Detached HEAD is handled safely: CurrentBranch() returns "", so only the target
// branch match can block deletion.
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
		return false, fmt.Errorf("failed to list remote branches for orphan check: %w", err)
	}
	if _, exists := remoteBranchNames[branchName]; exists {
		return false, nil
	}

	currentBranch, err := r.CurrentBranch()
	if err != nil {
		return false, fmt.Errorf("failed to determine current branch before orphan cleanup: %w", err)
	}
	// Detached HEAD returns an empty current branch; in that case no branch is
	// considered checked out and orphan cleanup can proceed.
	if currentBranch == branchName {
		slog.Debug("[DEBUG-GIT] skip orphaned branch cleanup: branch is currently checked out",
			"branch", branchName)
		return false, nil
	}

	if _, err := r.runGitCommand("branch", "-d", branchName); err != nil {
		return false, fmt.Errorf("failed to delete orphaned branch %q (may have unmerged commits): %w", branchName, err)
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
	remoteNames, err := r.listRemoteNames()
	if err != nil {
		return nil, err
	}

	names := map[string]struct{}{}
	if strings.TrimSpace(output) == "" {
		return names, nil
	}

	for line := range strings.SplitSeq(output, "\n") {
		refName := strings.TrimSpace(line)
		if refName == "" {
			continue
		}
		branchName, ok := stripRemotePrefix(refName, remoteNames)
		if !ok {
			continue
		}
		if branchName == "" || branchName == "HEAD" {
			continue
		}
		names[branchName] = struct{}{}
	}
	return names, nil
}

func (r *Repository) listRemoteNames() ([]string, error) {
	output, err := r.runGitCommand("remote")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == "" {
		return []string{}, nil
	}

	names := make([]string, 0, 4)
	for line := range strings.SplitSeq(output, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		// Prefer longer names first to handle nested remote names like "team/origin".
		return len(names[i]) > len(names[j])
	})
	return names, nil
}

func stripRemotePrefix(refName string, remoteNames []string) (string, bool) {
	for _, remoteName := range remoteNames {
		prefix := remoteName + "/"
		if !strings.HasPrefix(refName, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(refName, prefix)), true
	}
	// Fallback when remoteNames did not include the prefix (for example due to
	// stale/partial metadata): split once at the first slash and keep the branch
	// suffix to preserve compatibility with uncommon remote naming.
	parts := strings.SplitN(refName, "/", 2)
	if len(parts) != 2 {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

func hasAnyLiveUpstream(infos []branchTrackingInfo) bool {
	for _, info := range infos {
		if info.hasLiveUpstream() {
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
	if _, err := r.runGitCommand("checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to checkout new branch %q: %w", branchName, err)
	}
	return nil
}

// CheckoutDetachedHead switches the repository to detached HEAD state.
func (r *Repository) CheckoutDetachedHead() error {
	if _, err := r.runGitCommand("checkout", "--detach"); err != nil {
		return fmt.Errorf("failed to checkout detached HEAD: %w", err)
	}
	return nil
}

// DeleteLocalBranch deletes a local branch.
// When force is true, "-D" is used instead of "-d".
func (r *Repository) DeleteLocalBranch(branchName string, force bool) error {
	if err := ValidateBranchName(branchName); err != nil {
		return err
	}
	deleteFlag := "-d"
	if force {
		deleteFlag = "-D"
	}
	if _, err := r.runGitCommand("branch", deleteFlag, branchName); err != nil {
		return fmt.Errorf("failed to delete local branch %q: %w", branchName, err)
	}
	return nil
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
	if _, err := r.runGitCommand("pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull --ff-only failed: %w", err)
	}
	return nil
}

// CommitAll stages all changes and commits with the given message.
// NOTE: If "git commit" fails after a successful "git add -A", staged changes
// remain in the index for user inspection/retry.
func (r *Repository) CommitAll(message string) error {
	if strings.TrimSpace(message) == "" {
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

// Push pushes the current branch to its configured remote (defaults to "origin").
func (r *Repository) Push() error {
	remoteName, err := r.resolveRemoteName()
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	if _, err := r.runGitCommand("push", remoteName, "HEAD"); err != nil {
		return fmt.Errorf("git push %s HEAD failed: %w", remoteName, err)
	}
	return nil
}

// ResolveRemoteName returns the configured remote for the given branch.
// Falls back to "origin" when no remote is explicitly configured (git config
// key not found — exit code 1). Returns an error for unexpected failures
// (I/O, permissions, corrupt config) so the caller can abort instead of
// silently pushing to a wrong remote.
func ResolveRemoteName(workDir, branch string) (string, error) {
	output, err := RunGitCLIPublic(workDir, []string{
		"config", fmt.Sprintf("branch.%s.remote", branch),
	})
	if err == nil {
		if name := strings.TrimSpace(string(output)); name != "" {
			return name, nil
		}
		// Config returned successfully but with empty value — fall back to "origin".
		return "origin", nil
	}
	// git config returns exit code 1 when key is not found — this is expected.
	if IsGitConfigKeyNotFound(err) {
		return "origin", nil
	}
	// Other errors (I/O, permissions, corrupt config) must be propagated
	// to prevent pushing to an incorrect remote.
	return "", fmt.Errorf("failed to resolve remote name for branch %q: %w", branch, err)
}

// resolveRemoteName resolves the remote for the Repository's current branch.
func (r *Repository) resolveRemoteName() (string, error) {
	branchOutput, err := r.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to determine current branch: %w", err)
	}
	return ResolveRemoteName(r.path, strings.TrimSpace(string(branchOutput)))
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
		// NOTE: Exit code extraction (as done in IsGitConfigKeyNotFound) could
		// reduce dependence on locale-specific strings here, but upstream errors
		// share exit code 128 with other fatal errors, so message matching remains
		// the most reliable approach for this specific case.
		if IsNoUpstreamError(err.Error()) {
			slog.Debug("[DEBUG-GIT] HasUnpushedCommits: no upstream tracking branch",
				"path", r.path, "error", err)
			return false, nil
		}
		// Other errors (disk I/O, permission, etc.) are propagated.
		return false, fmt.Errorf("HasUnpushedCommits: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

// IsNoUpstreamError reports whether errMsg indicates that no upstream branch
// is configured. All upstream-missing detection patterns are consolidated here
// to avoid scattered string-matching across the codebase (DRY).
//
// Known git error patterns (version/locale dependent), in switch-case order:
//  1. "no upstream configured for branch 'xxx'"
//  2. "no upstream branch"
//  3. "has no upstream"
//  4. "set the remote as upstream"
//  5. "does not point to a branch" (detached HEAD)
//  6. "does not point to a commit" (rev-list @{u} with no upstream)
//  7. "no such ref" / "not a valid ref" AND contains @{u}/@{upstream} token
//     (AND condition prevents false positives on unrelated ref errors)
func IsNoUpstreamError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "no upstream configured"),
		strings.Contains(lower, "no upstream branch"),
		strings.Contains(lower, "has no upstream"),
		strings.Contains(lower, "set the remote as upstream"),
		strings.Contains(lower, "does not point to a branch"),
		strings.Contains(lower, "does not point to a commit"):
		return true
	case containsUpstreamToken(lower) &&
		(strings.Contains(lower, "no such ref") || strings.Contains(lower, "not a valid ref")):
		return true
	default:
		return false
	}
}

func containsUpstreamToken(lowerErrMsg string) bool {
	// Match only explicit upstream ref tokens to avoid over-matching unrelated
	// errors that happen to include the word "upstream".
	// Caller must pass an already-lowercased message.
	return strings.Contains(lowerErrMsg, "@{u}") || strings.Contains(lowerErrMsg, "@{upstream}")
}

// IsGitConfigKeyNotFound returns true when the error is a git config
// "key not found" exit (exit code 1). This distinguishes the normal
// "no config entry" case from unexpected failures (I/O, permissions).
//
// IMPORTANT: This function should only be called on errors from "git config"
// commands. Exit code 1 has different meanings for other git subcommands
// (e.g., git diff uses exit code 1 to indicate differences exist). Using
// this function on non-config errors will produce incorrect results.
//
// Checks both the unwrapped *exec.ExitError (primary path via %w chain)
// and the string representation as a defensive fallback.
func IsGitConfigKeyNotFound(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 1
	}
	// runGitCLI now wraps errors with %w, so errors.As should unwrap
	// *exec.ExitError in the primary path above. This string fallback
	// covers edge cases where the error chain might be reconstructed
	// without %w (e.g., by third-party wrappers or serialization).
	msg := err.Error()
	return strings.HasSuffix(msg, "exit status 1")
}
