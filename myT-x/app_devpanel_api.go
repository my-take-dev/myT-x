package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	gitpkg "myT-x/internal/git"
)

// devPanelMaxFileSize is the maximum file size returned by DevPanelReadFile (1 MB).
const devPanelMaxFileSize int64 = 1 << 20

// devPanelMaxDirEntries is the maximum number of directory entries returned by DevPanelListDir.
const devPanelMaxDirEntries = 5000

// devPanelBinaryProbeSize is the number of bytes scanned to detect binary content.
const devPanelBinaryProbeSize = 8192

// devPanelMaxDiffSize is the maximum diff output size (500 KB).
const devPanelMaxDiffSize = 500 * 1024

// devPanelMaxUntrackedFilePaths is the maximum number of untracked paths parsed
// from git ls-files output to prevent unbounded memory growth.
const devPanelMaxUntrackedFilePaths = 10000

// devPanelMaxGitLogCount is the maximum number of commits returned by DevPanelGitLog.
const devPanelMaxGitLogCount = 1000

// devPanelExcludedDirs contains directory names excluded from listings.
var devPanelExcludedDirs = []string{".git", "node_modules"}

// resolveSessionDir resolves a directory path for a session.
// When preferWorktree is true, returns the worktree path for worktree sessions (working directory).
// When preferWorktree is false, returns the repo path for worktree sessions (git operations).
// For regular sessions, both modes return root_path.
func (a *App) resolveSessionDir(sessionName string, preferWorktree bool) (string, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return "", err
	}

	snapshots := sessions.Snapshot()
	for _, s := range snapshots {
		if s.Name != sessionName {
			continue
		}
		if s.Worktree != nil {
			if preferWorktree && s.Worktree.Path != "" {
				return s.Worktree.Path, nil
			}
			if !preferWorktree && s.Worktree.RepoPath != "" {
				return s.Worktree.RepoPath, nil
			}
		}
		if s.RootPath != "" {
			return s.RootPath, nil
		}
		return "", fmt.Errorf("session %q has no root path configured", sessionName)
	}
	return "", fmt.Errorf("session %q not found", sessionName)
}

// resolveSessionWorkDir resolves the working directory for a session.
// For worktree sessions, returns the worktree path; otherwise returns root_path.
func (a *App) resolveSessionWorkDir(sessionName string) (string, error) {
	return a.resolveSessionDir(sessionName, true)
}

// resolveAndValidatePath resolves dirPath relative to rootDir and validates
// that the result stays within rootDir boundaries.
func resolveAndValidatePath(rootDir, relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	// filepath.IsLocal rejects "..", absolute paths, and OS-reserved names (e.g. NUL on Windows).
	if !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("path is not local (absolute, traversal, or reserved): %s", relPath)
	}

	// Resolve rootDir symlinks first for accurate containment check.
	resolvedRoot, rootErr := filepath.EvalSymlinks(rootDir)
	if rootErr != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", rootErr)
	}

	absPath := filepath.Join(resolvedRoot, cleaned)

	// Resolve symlinks and verify containment.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("path does not exist: %s", relPath)
		}
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	if !isPathWithinBase(resolved, resolvedRoot) {
		return "", fmt.Errorf("path escapes root directory: %s", relPath)
	}
	return resolved, nil
}

// DevPanelListDir returns the contents of a directory within a session's working directory.
// Directories are listed first, sorted alphabetically. Lazy-loading friendly.
func (a *App) DevPanelListDir(sessionName string, dirPath string) ([]FileEntry, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}

	rootDir, err := a.resolveSessionWorkDir(sessionName)
	if err != nil {
		return nil, err
	}

	// For root listing, dirPath is empty or ".".
	targetDir := rootDir
	if dirPath != "" && dirPath != "." {
		resolved, resolveErr := resolveAndValidatePath(rootDir, dirPath)
		if resolveErr != nil {
			return nil, resolveErr
		}
		targetDir = resolved
	}

	entries, readErr := os.ReadDir(targetDir)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read directory: %w", readErr)
	}

	var dirs []FileEntry
	var files []FileEntry
	count := 0
	for _, entry := range entries {
		if count >= devPanelMaxDirEntries {
			slog.Warn("[DEVPANEL] directory entry limit reached",
				"dir", targetDir, "limit", devPanelMaxDirEntries)
			break
		}

		name := entry.Name()

		// Skip excluded directories.
		if entry.IsDir() && slices.Contains(devPanelExcludedDirs, name) {
			continue
		}

		// Dotfiles are intentionally visible in the developer panel.

		relPath, relErr := filepath.Rel(rootDir, filepath.Join(targetDir, name))
		if relErr != nil {
			slog.Debug("[DEVPANEL] failed to compute relative path", "name", name, "error", relErr)
			continue
		}
		// Normalize separators to forward slash for frontend consistency.
		relPath = filepath.ToSlash(relPath)

		fe := FileEntry{
			Name:  name,
			Path:  relPath,
			IsDir: entry.IsDir(),
		}

		if !entry.IsDir() {
			if info, infoErr := entry.Info(); infoErr == nil {
				fe.Size = info.Size()
			}
		}

		if entry.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
		count++
	}

	// Sort: directories first (alphabetically), then files (alphabetically).
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	result := make([]FileEntry, 0, len(dirs)+len(files))
	result = append(result, dirs...)
	result = append(result, files...)
	return result, nil
}

// DevPanelReadFile reads a file within a session's working directory.
// Returns the file content with metadata. Files exceeding 1MB are truncated.
// Binary files are detected by scanning the first 8KB for NULL bytes.
func (a *App) DevPanelReadFile(sessionName string, filePath string) (FileContent, error) {
	sessionName = strings.TrimSpace(sessionName)
	filePath = strings.TrimSpace(filePath)
	if sessionName == "" {
		return FileContent{}, errors.New("session name is required")
	}
	if filePath == "" {
		return FileContent{}, errors.New("file path is required")
	}

	rootDir, err := a.resolveSessionWorkDir(sessionName)
	if err != nil {
		return FileContent{}, err
	}

	resolved, resolveErr := resolveAndValidatePath(rootDir, filePath)
	if resolveErr != nil {
		return FileContent{}, resolveErr
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return FileContent{}, fmt.Errorf("failed to stat file: %w", statErr)
	}
	if info.IsDir() {
		return FileContent{}, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	result := FileContent{
		Path: filepath.ToSlash(filePath),
		Size: info.Size(),
	}

	f, openErr := os.Open(resolved)
	if openErr != nil {
		return FileContent{}, fmt.Errorf("failed to open file: %w", openErr)
	}
	defer f.Close()

	// Binary detection: read first probe bytes and scan for NULL bytes.
	probeSize := min(int64(devPanelBinaryProbeSize), info.Size())
	probe := make([]byte, probeSize)
	probeN, probeReadErr := io.ReadFull(f, probe)
	if probeReadErr != nil && !errors.Is(probeReadErr, io.ErrUnexpectedEOF) {
		return FileContent{}, fmt.Errorf("failed to read file probe: %w", probeReadErr)
	}
	probe = probe[:probeN]

	if bytes.IndexByte(probe, 0) >= 0 {
		result.Binary = true
		result.Content = ""
		return result, nil
	}

	// Read remainder up to max file size using LimitReader.
	// Read one extra byte beyond the limit to detect truncation.
	remainLimit := max(devPanelMaxFileSize-int64(probeN), 0)
	remainder, readErr := io.ReadAll(io.LimitReader(f, remainLimit+1))
	if readErr != nil {
		return FileContent{}, fmt.Errorf("failed to read file: %w", readErr)
	}

	data := append(probe, remainder...)

	// Truncate if total exceeds devPanelMaxFileSize.
	if int64(len(data)) > devPanelMaxFileSize {
		data = data[:devPanelMaxFileSize]
		result.Truncated = true
	}

	result.Content = string(data)
	result.LineCount = strings.Count(result.Content, "\n") + 1
	return result, nil
}

// resolveSessionRepoDir resolves the git repository directory for a session.
// For worktree sessions, returns the repo_path (original repository).
// For regular sessions with root_path, returns root_path.
func (a *App) resolveSessionRepoDir(sessionName string) (string, error) {
	return a.resolveSessionDir(sessionName, false)
}

// DevPanelGitLog returns the commit history for a session's repository.
// Results include parent hashes for graph rendering.
func (a *App) DevPanelGitLog(sessionName string, maxCount int, allBranches bool) ([]GitGraphCommit, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}
	if maxCount <= 0 {
		maxCount = 100
	}
	if maxCount > devPanelMaxGitLogCount {
		maxCount = devPanelMaxGitLogCount
	}

	repoDir, err := a.resolveSessionRepoDir(sessionName)
	if err != nil {
		return nil, err
	}

	if !gitpkg.IsGitRepository(repoDir) {
		return nil, fmt.Errorf("not a git repository: %s", repoDir)
	}

	// Use NUL-delimited format for safe parsing.
	// Fields: full_hash, short_hash, parents, subject, author_name, author_date_iso, decorations
	args := []string{
		"log",
		"--format=%H%x00%h%x00%P%x00%s%x00%an%x00%aI%x00%D",
		fmt.Sprintf("-n%d", maxCount),
	}
	if allBranches {
		args = append(args, "--all")
	}

	output, gitErr := gitpkg.RunGitCLIPublic(repoDir, args)
	if gitErr != nil {
		return nil, fmt.Errorf("git log failed: %w", gitErr)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]GitGraphCommit, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\x00", 7)
		if len(fields) < 6 {
			slog.Debug("[DEVPANEL] skipping malformed git log line", "line", line)
			continue
		}

		commit := GitGraphCommit{
			FullHash:   fields[0],
			Hash:       fields[1],
			Subject:    fields[3],
			AuthorName: fields[4],
			AuthorDate: fields[5],
		}

		// Parse parent hashes (space-separated full hashes).
		if parentStr := strings.TrimSpace(fields[2]); parentStr != "" {
			commit.Parents = strings.Fields(parentStr)
		}

		// Parse refs/decorations.
		if len(fields) >= 7 && strings.TrimSpace(fields[6]) != "" {
			rawRefs := strings.Split(fields[6], ",")
			refs := make([]string, 0, len(rawRefs))
			for _, r := range rawRefs {
				r = strings.TrimSpace(r)
				// Strip "HEAD -> " prefix.
				r = strings.TrimPrefix(r, "HEAD -> ")
				if r != "" && r != "HEAD" {
					refs = append(refs, r)
				}
			}
			// Always include HEAD if present.
			if strings.Contains(fields[6], "HEAD") {
				refs = append([]string{"HEAD"}, refs...)
			}
			commit.Refs = refs
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// DevPanelGitStatus returns the working tree status for a session's repository.
func (a *App) DevPanelGitStatus(sessionName string) (GitStatusResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return GitStatusResult{}, errors.New("session name is required")
	}

	workDir, err := a.resolveSessionWorkDir(sessionName)
	if err != nil {
		return GitStatusResult{}, err
	}

	if !gitpkg.IsGitRepository(workDir) {
		return GitStatusResult{}, fmt.Errorf("not a git repository")
	}

	result := GitStatusResult{}

	// Get branch name.
	branchOutput, branchErr := gitpkg.RunGitCLIPublic(workDir, []string{"rev-parse", "--abbrev-ref", "HEAD"})
	if branchErr == nil {
		result.Branch = strings.TrimSpace(string(branchOutput))
	} else {
		slog.Debug("[DEVPANEL] failed to resolve branch name (e.g. detached HEAD)",
			"session", sessionName, "error", branchErr)
	}

	// Get status (porcelain format).
	statusOutput, statusErr := gitpkg.RunGitCLIPublic(workDir, []string{"status", "--porcelain", "-b"})
	if statusErr != nil {
		return result, fmt.Errorf("git status failed: %w", statusErr)
	}

	for line := range strings.SplitSeq(string(statusOutput), "\n") {
		if len(line) < 4 {
			continue
		}
		// Skip branch header line.
		if strings.HasPrefix(line, "## ") {
			continue
		}
		indexStatus := line[0]
		workTreeStatus := line[1]
		filePath := strings.TrimSpace(line[3:])

		// Staged changes.
		if indexStatus != ' ' && indexStatus != '?' {
			result.Staged = append(result.Staged, filePath)
		}
		// Modified (unstaged) changes.
		if workTreeStatus == 'M' || workTreeStatus == 'D' {
			result.Modified = append(result.Modified, filePath)
		}
		// Untracked files.
		if indexStatus == '?' && workTreeStatus == '?' {
			result.Untracked = append(result.Untracked, filePath)
		}
	}

	// Get ahead/behind counts.
	revListOutput, revListErr := gitpkg.RunGitCLIPublic(workDir,
		[]string{"rev-list", "--left-right", "--count", "@{u}...HEAD"})
	if revListErr == nil {
		parts := strings.Fields(strings.TrimSpace(string(revListOutput)))
		if len(parts) == 2 {
			if _, scanErr := fmt.Sscanf(parts[0], "%d", &result.Behind); scanErr != nil {
				slog.Debug("[DEVPANEL] failed to parse behind count", "raw", parts[0], "error", scanErr)
			}
			if _, scanErr := fmt.Sscanf(parts[1], "%d", &result.Ahead); scanErr != nil {
				slog.Debug("[DEVPANEL] failed to parse ahead count", "raw", parts[1], "error", scanErr)
			}
		}
	}

	return result, nil
}

// DevPanelCommitDiff returns the unified diff for a specific commit.
func (a *App) DevPanelCommitDiff(sessionName string, commitHash string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	commitHash = strings.TrimSpace(commitHash)
	if sessionName == "" {
		return "", errors.New("session name is required")
	}
	if commitHash == "" {
		return "", errors.New("commit hash is required")
	}

	if err := gitpkg.ValidateCommitish(commitHash); err != nil {
		return "", fmt.Errorf("invalid commit hash: %w", err)
	}

	repoDir, err := a.resolveSessionRepoDir(sessionName)
	if err != nil {
		return "", err
	}

	if !gitpkg.IsGitRepository(repoDir) {
		return "", fmt.Errorf("not a git repository")
	}

	// SECURITY: commitHash is validated by ValidateCommitish above.
	// --root ensures root commits (no parent) also produce diff output.
	output, gitErr := gitpkg.RunGitCLIPublic(repoDir, []string{"diff-tree", "--root", "-p", commitHash, "--"})
	if gitErr != nil {
		return "", fmt.Errorf("git diff-tree failed: %w", gitErr)
	}

	diff := string(output)
	if len(diff) > devPanelMaxDiffSize {
		diff = diff[:devPanelMaxDiffSize] + "\n... (diff truncated)"
	}
	return diff, nil
}

// devPanelMaxUntrackedFileSize is the maximum size of an individual untracked file
// included in the working diff (100 KB). Larger files are skipped to avoid memory bloat.
const devPanelMaxUntrackedFileSize int64 = 100 * 1024

// DevPanelWorkingDiff returns the unified diff of working changes (staged + unstaged) vs HEAD,
// plus synthetic diffs for untracked (new) files.
func (a *App) DevPanelWorkingDiff(sessionName string) (WorkingDiffResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return WorkingDiffResult{}, errors.New("session name is required")
	}

	workDir, err := a.resolveSessionWorkDir(sessionName)
	if err != nil {
		return WorkingDiffResult{}, err
	}

	if !gitpkg.IsGitRepository(workDir) {
		// Not a git repository -- return empty result, no error.
		return WorkingDiffResult{}, nil
	}

	isFreshRepo, headProbeErr := detectFreshRepoState(workDir, gitpkg.RunGitCLIPublic)
	if headProbeErr != nil {
		return WorkingDiffResult{}, headProbeErr
	}

	var output []byte
	diffLoadIncomplete := false
	if isFreshRepo {
		slog.Debug("[DEVPANEL] repository has no commits yet; using staged-only diff path")
		var gitErr error
		output, gitErr = gitpkg.RunGitCLIPublic(workDir, []string{
			"-c", "core.quotepath=false",
			"diff", "--cached", "--no-color", "--",
		})
		if gitErr != nil {
			// Continue with untracked-only fallback while marking the response truncated.
			diffLoadIncomplete = true
			slog.Warn("[DEVPANEL] failed to load staged diff for fresh repo", "error", gitErr)
		}
	} else {
		var gitErr error
		output, gitErr = gitpkg.RunGitCLIPublic(workDir, []string{
			"-c", "core.quotepath=false",
			"diff", "HEAD", "--no-color", "--",
		})
		if gitErr != nil {
			return WorkingDiffResult{}, fmt.Errorf("git diff HEAD failed: %w", gitErr)
		}
	}

	raw := string(output)
	truncated := diffLoadIncomplete
	if len(raw) > devPanelMaxDiffSize {
		raw = raw[:devPanelMaxDiffSize]
		// IMP-04: Remove the last incomplete diff block to prevent partial entry
		// corruption. Find the last "diff --git " boundary and truncate there.
		if lastIdx := strings.LastIndex(raw, "diff --git "); lastIdx > 0 {
			raw = raw[:lastIdx]
		}
		truncated = true
	}

	files := parseWorkingDiff(raw)

	// Collect untracked files via git ls-files.
	untrackedFiles, hasLsErr := collectUntrackedFiles(workDir)
	if hasLsErr {
		truncated = true
	}
	// consumedSize tracks the assembled payload length after any main diff truncation.
	// This intentionally starts from len(raw), not len(output), because budget checks
	// must match what is actually returned to the frontend.
	consumedSize := len(raw)
	for _, relPath := range untrackedFiles {
		remainingBudget := devPanelMaxDiffSize - consumedSize
		if remainingBudget <= 0 {
			truncated = true
			break
		}

		entries, budgetExceeded := buildUntrackedFileDiffsWithBudget(workDir, relPath, remainingBudget)
		for _, entry := range entries {
			consumedSize += len(entry.Diff)
			files = append(files, entry)
		}
		if budgetExceeded {
			truncated = true
			break
		}
	}

	totalAdded := 0
	totalDeleted := 0
	for _, f := range files {
		totalAdded += f.Additions
		totalDeleted += f.Deletions
	}

	return WorkingDiffResult{
		Files:        files,
		TotalAdded:   totalAdded,
		TotalDeleted: totalDeleted,
		Truncated:    truncated,
	}, nil
}

func detectFreshRepoState(workDir string, runGit func(string, []string) ([]byte, error)) (bool, error) {
	if runGit == nil {
		return false, errors.New("git runner is required")
	}

	_, headErr := runGit(workDir, []string{"rev-parse", "--verify", "--quiet", "HEAD"})
	if headErr == nil {
		return false, nil
	}

	statusOutput, statusErr := runGit(workDir, []string{"status", "--porcelain", "--branch"})
	if statusErr != nil {
		return false, fmt.Errorf("failed to verify HEAD state: %w", errors.Join(headErr, statusErr))
	}
	if isFreshRepoStatusOutput(statusOutput) {
		return true, nil
	}

	// HEAD verification can fail for reasons other than "no commits yet"
	// (for example repository corruption or execution issues). Do not silently
	// downgrade those cases to fresh-repo fallback.
	return false, fmt.Errorf("failed to verify HEAD commit: %w", headErr)
}

func isFreshRepoStatusOutput(statusOutput []byte) bool {
	if len(statusOutput) == 0 {
		return false
	}

	normalized := strings.ReplaceAll(string(statusOutput), "\r\n", "\n")
	for line := range strings.SplitSeq(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "## No commits yet on ") ||
			strings.HasPrefix(line, "## Initial commit on ")
	}
	return false
}

// collectUntrackedFiles returns the list of untracked file paths from git ls-files.
func collectUntrackedFiles(workDir string) ([]string, bool) {
	// git ls-files --others --exclude-standard is preferred over git status -uall:
	// - Always returns individual file paths (not directory entries)
	// - Behavior is stable across git versions
	// - Correctly applies .gitignore rules
	output, err := gitpkg.RunGitCLIPublic(workDir, []string{
		"-c", "core.quotepath=false",
		"ls-files", "--others", "--exclude-standard", "-z",
	})
	if err != nil {
		slog.Warn("[DEVPANEL] git ls-files for untracked files failed", "error", err)
		return nil, true
	}

	return parseNULSeparatedGitPaths(output), false
}

func parseNULSeparatedGitPaths(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}

	entries := strings.Split(string(raw), "\x00")
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if len(paths) >= devPanelMaxUntrackedFilePaths {
			slog.Warn("[DEVPANEL] untracked path limit reached",
				"limit", devPanelMaxUntrackedFilePaths,
				"dropped", len(entries)-len(paths))
			break
		}
		paths = append(paths, filepath.ToSlash(entry))
	}
	return paths
}

// buildUntrackedFileDiffs creates synthetic diff entries for an untracked path.
// If the path is a directory, it recurses into it and produces entries for each file.
// Returns nil for binary/too-large/unreadable files.
func buildUntrackedFileDiffs(workDir, relPath string) []WorkingDiffFile {
	results, _ := buildUntrackedFileDiffsWithBudget(workDir, relPath, -1)
	return results
}

func buildUntrackedFileDiffsWithBudget(workDir, relPath string, remainingBudget int) ([]WorkingDiffFile, bool) {
	resolvedBase, baseErr := filepath.EvalSymlinks(workDir)
	if baseErr != nil {
		slog.Warn("[DEVPANEL] failed to resolve workDir while parsing untracked files", "error", baseErr)
		return nil, false
	}

	absPath := filepath.Join(workDir, filepath.FromSlash(relPath))

	info, err := os.Lstat(absPath)
	if err != nil {
		slog.Debug("[DEVPANEL] failed to stat untracked path", "path", relPath, "error", err)
		return nil, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Debug("[DEVPANEL] skipping symlink in untracked files", "path", relPath)
		return nil, false
	}

	if !info.IsDir() {
		// Pass Lstat metadata to avoid redundant stat calls per file.
		entry := buildUntrackedFileDiffSingleWithResolvedBase(workDir, relPath, resolvedBase, info)
		if entry == nil {
			return nil, false
		}
		if remainingBudget >= 0 && len(entry.Diff) > remainingBudget {
			return nil, true
		}
		return []WorkingDiffFile{*entry}, false
	}

	// Directory: walk recursively and collect individual files.
	var results []WorkingDiffFile
	budgetExceeded := false
	walkErr := filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("[DEVPANEL] failed to walk untracked path", "path", path, "error", err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// Skip excluded directories inside the walk.
			if slices.Contains(devPanelExcludedDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if remainingBudget == 0 {
			budgetExceeded = true
			return fs.SkipAll
		}
		rel, relErr := filepath.Rel(workDir, path)
		if relErr != nil {
			return nil
		}
		normalized := filepath.ToSlash(rel)

		info, infoErr := d.Info()
		if infoErr != nil {
			slog.Debug("[DEVPANEL] failed to read file info while walking untracked files",
				"path", normalized, "error", infoErr)
			return nil
		}
		entry := buildUntrackedFileDiffSingleWithResolvedBase(workDir, normalized, resolvedBase, info)
		if entry != nil {
			if remainingBudget >= 0 && len(entry.Diff) > remainingBudget {
				budgetExceeded = true
				return fs.SkipAll
			}
			results = append(results, *entry)
			if remainingBudget >= 0 {
				remainingBudget -= len(entry.Diff)
			}
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
		slog.Warn("[DEVPANEL] WalkDir completed with error", "path", absPath, "error", walkErr)
	}
	return results, budgetExceeded
}

// buildUntrackedFileDiffSingle creates a synthetic diff entry for a single untracked file.
// Returns nil if the file is binary, too large, or cannot be read.
// Accepts an optional pre-fetched os.FileInfo (expected to come from os.Lstat)
// to avoid redundant filesystem metadata lookups.
func buildUntrackedFileDiffSingle(workDir, relPath string, cachedInfo ...os.FileInfo) *WorkingDiffFile {
	resolvedBase, baseErr := filepath.EvalSymlinks(workDir)
	if baseErr != nil {
		slog.Warn("[DEVPANEL] failed to resolve workDir while parsing untracked file", "error", baseErr)
		return nil
	}
	return buildUntrackedFileDiffSingleWithResolvedBase(workDir, relPath, resolvedBase, cachedInfo...)
}

func buildUntrackedFileDiffSingleWithResolvedBase(workDir, relPath, resolvedBase string, cachedInfo ...os.FileInfo) *WorkingDiffFile {
	absPath := filepath.Join(workDir, filepath.FromSlash(relPath))

	// Lexical guard before any file access.
	if !isPathWithinBase(absPath, workDir) {
		slog.Warn("[DEVPANEL] untracked path escapes workDir", "path", relPath)
		return nil
	}

	// Optional fast-path metadata from caller. At most one value is used.
	// Expected contract: cachedInfo[0] is an os.Lstat result for absPath.
	var info os.FileInfo
	if len(cachedInfo) > 0 && cachedInfo[0] != nil {
		info = cachedInfo[0]
	} else {
		lstatInfo, err := os.Lstat(absPath)
		if err != nil {
			slog.Debug("[DEVPANEL] failed to lstat untracked file", "path", relPath, "error", err)
			return nil
		}
		info = lstatInfo
	}
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Debug("[DEVPANEL] skipping symlink in untracked files", "path", relPath)
		return nil
	}
	if info.IsDir() {
		return nil
	}

	// Filesystem guard: resolve both base and target path and ensure containment
	// after symlink resolution (covers path traversal via symlink hops).
	resolvedAbs, evalErr := filepath.EvalSymlinks(absPath)
	if evalErr != nil {
		slog.Debug("[DEVPANEL] failed to resolve untracked path", "path", relPath, "error", evalErr)
		return nil
	}
	if !isPathWithinBase(resolvedAbs, resolvedBase) {
		slog.Warn("[DEVPANEL] untracked path escapes workDir after symlink resolution", "path", relPath)
		return nil
	}

	// Skip files that are too large.
	if info.Size() > devPanelMaxUntrackedFileSize {
		slog.Debug("[DEVPANEL] skipping large untracked file", "path", relPath, "size", info.Size())
		return nil
	}

	f, err := os.Open(resolvedAbs)
	if err != nil {
		slog.Debug("[DEVPANEL] failed to open untracked file", "path", relPath, "error", err)
		return nil
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Debug("[DEVPANEL] failed to close untracked file", "path", relPath, "error", closeErr)
		}
	}()

	// Defensive hard cap: protects against TOCTOU where file size grows between
	// metadata check and read time.
	data, err := io.ReadAll(io.LimitReader(f, devPanelMaxUntrackedFileSize+1))
	if err != nil {
		slog.Debug("[DEVPANEL] failed to read untracked file", "path", relPath, "error", err)
		return nil
	}
	if int64(len(data)) > devPanelMaxUntrackedFileSize {
		slog.Debug("[DEVPANEL] skipping untracked file that exceeded size limit during read",
			"path", relPath, "limit", devPanelMaxUntrackedFileSize)
		return nil
	}

	// Binary detection: scan for NULL bytes in first probe bytes.
	probeSize := min(len(data), devPanelBinaryProbeSize)
	if bytes.IndexByte(data[:probeSize], 0) >= 0 {
		return nil
	}

	content := string(data)
	// Normalize Windows line endings for consistent line splitting.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Build synthetic unified diff.
	var sb strings.Builder
	normalizedPath := filepath.ToSlash(relPath)

	// IMP-06: Quote paths containing spaces or control characters to produce
	// Git-compatible diff headers. Git uses C-style quoting for such paths.
	diffPath := normalizedPath
	if needsGitQuoting(diffPath) {
		diffPath = strconv.Quote(diffPath)
	}

	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", diffPath, diffPath)
	sb.WriteString("new file mode 100644\n")
	sb.WriteString("index 0000000..0000000\n")
	sb.WriteString("--- /dev/null\n")
	fmt.Fprintf(&sb, "+++ b/%s\n", diffPath)
	// SUG-06: Empty files (len(lines) == 0) produce no hunk header or body,
	// matching Git's behavior for new empty files.
	if len(lines) == 0 {
		return &WorkingDiffFile{
			Path:      normalizedPath,
			OldPath:   "",
			Status:    WorkingDiffStatusUntracked,
			Additions: 0,
			Deletions: 0,
			Diff:      sb.String(),
		}
	}
	fmt.Fprintf(&sb, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		sb.WriteByte('+')
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	// SUG-07: New (untracked) files have no previous path; OldPath is empty.
	return &WorkingDiffFile{
		Path:      normalizedPath,
		OldPath:   "",
		Status:    WorkingDiffStatusUntracked,
		Additions: len(lines),
		Deletions: 0,
		Diff:      sb.String(),
	}
}

// parseWorkingDiff parses a unified diff output into per-file WorkingDiffFile entries.
func parseWorkingDiff(raw string) []WorkingDiffFile {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	// NOTE: This parser handles standard unified diff blocks ("diff --git ...").
	// Combined diff formats (e.g. merge combined hunks) are out of scope.

	// Normalize Windows line endings for robust path extraction.
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	blocks := splitUnifiedDiffBlocks(raw)
	if len(blocks) == 0 {
		return nil
	}

	var files []WorkingDiffFile

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}
		header := strings.TrimPrefix(lines[0], "diff --git ")
		if header == lines[0] {
			continue
		}

		oldPath, newPath, ok := parseDiffHeaderPaths(header)
		if !ok {
			slog.Debug("[DEVPANEL] skipping diff block with malformed header", "header", header)
			continue
		}

		file := WorkingDiffFile{
			Path:    filepath.ToSlash(newPath),
			OldPath: filepath.ToSlash(oldPath),
			Diff:    block,
		}

		// Detect status and count additions/deletions.
		file.Status = WorkingDiffStatusModified // default
		for _, line := range lines {
			if strings.HasPrefix(line, "new file mode") {
				file.Status = WorkingDiffStatusAdded
			} else if strings.HasPrefix(line, "deleted file mode") {
				file.Status = WorkingDiffStatusDeleted
			} else if after, ok := strings.CutPrefix(line, "rename from "); ok {
				if renamedFrom, decoded := decodeGitPathLiteral(after); decoded {
					file.OldPath = filepath.ToSlash(renamedFrom)
				}
				file.Status = WorkingDiffStatusRenamed
			} else if after, ok := strings.CutPrefix(line, "rename to "); ok {
				if renamedTo, decoded := decodeGitPathLiteral(after); decoded {
					file.Path = filepath.ToSlash(renamedTo)
				}
				file.Status = WorkingDiffStatusRenamed
			} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				file.Additions++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				file.Deletions++
			}
		}

		files = append(files, file)
	}

	return files
}

func splitUnifiedDiffBlocks(raw string) []string {
	const marker = "diff --git "
	var blocks []string

	start := -1
	searchFrom := 0
	for {
		idx := strings.Index(raw[searchFrom:], marker)
		if idx < 0 {
			break
		}
		idx += searchFrom
		// Treat "diff --git " as a block header only when it appears at line start.
		if idx > 0 && raw[idx-1] != '\n' {
			searchFrom = idx + len(marker)
			continue
		}
		if start >= 0 {
			blocks = append(blocks, strings.TrimRight(raw[start:idx], "\n"))
		}
		start = idx
		searchFrom = idx + len(marker)
	}

	if start >= 0 {
		blocks = append(blocks, strings.TrimRight(raw[start:], "\n"))
	}

	return blocks
}

// parseDiffHeaderPaths extracts old/new paths from a "diff --git" header line.
// Assumes default git diff format with "a/" and "b/" prefixes (NOT --no-prefix).
//
// IMP-01: For renames (old != new), the symmetry heuristic cannot match because
// the two path segments differ. The fallback splits at the first " b/", which may
// produce incorrect results when the old path itself contains " b/" (e.g.,
// "a/lib b/foo b/lib c/bar"). Callers must apply rename from/rename to lines
// (parsed in parseWorkingDiff) to override these potentially inaccurate paths.
func parseDiffHeaderPaths(header string) (oldPath, newPath string, ok bool) {
	header = strings.TrimSpace(header)

	// Quoted paths (handles special chars, spaces, non-ASCII).
	if strings.HasPrefix(header, "\"") {
		return parseDiffHeaderPathsQuoted(header)
	}

	// Unquoted paths: header format is "a/<old> b/<new>".
	if !strings.HasPrefix(header, "a/") {
		return "", "", false
	}

	// For non-rename diffs (majority), old == new, so header = "a/<P> b/<P>".
	// Use path-length symmetry to find the split point.
	// Even total lengths (after removing "a/") intentionally fail this check and
	// fall back to separator-based parsing below.
	content := header[2:] // strip "a/"
	if len(content) >= 3 {
		pathLen := (len(content) - 3) / 2
		if 2*pathLen+3 == len(content) {
			candidate := content[:pathLen]
			if content[pathLen:pathLen+3] == " b/" && content[pathLen+3:] == candidate {
				return candidate, candidate, true
			}
		}
	}

	// Fallback for renames or asymmetric paths: split at first " b/".
	// For renames, the result here is a best-effort guess that will be
	// overridden by "rename from"/"rename to" lines in parseWorkingDiff.
	sepIdx := strings.Index(header, " b/")
	if sepIdx < 0 {
		return "", "", false
	}
	return header[2:sepIdx], header[sepIdx+3:], true
}

// parseDiffHeaderPathsQuoted parses diff header paths where at least one path is quoted.
func parseDiffHeaderPathsQuoted(header string) (oldPath string, newPath string, ok bool) {
	oldSpec, rest, specOk := takeDiffHeaderPathSpec(header)
	if !specOk {
		return "", "", false
	}

	newSpec, _, specOk := takeDiffHeaderPathSpec(rest)
	if !specOk {
		return "", "", false
	}

	oldLiteral, decOk := decodeGitPathLiteral(oldSpec)
	if !decOk || !strings.HasPrefix(oldLiteral, "a/") {
		return "", "", false
	}

	newLiteral, decOk := decodeGitPathLiteral(newSpec)
	if !decOk || !strings.HasPrefix(newLiteral, "b/") {
		return "", "", false
	}

	return strings.TrimPrefix(oldLiteral, "a/"), strings.TrimPrefix(newLiteral, "b/"), true
}

func takeDiffHeaderPathSpec(input string) (pathSpec string, rest string, ok bool) {
	trimmed := strings.TrimLeft(input, " \t")
	if trimmed == "" {
		return "", "", false
	}

	if trimmed[0] != '"' {
		end := strings.IndexAny(trimmed, " \t")
		if end == -1 {
			return trimmed, "", true
		}
		return trimmed[:end], trimmed[end:], true
	}

	escaped := false
	for i := 1; i < len(trimmed); i++ {
		ch := trimmed[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return trimmed[:i+1], trimmed[i+1:], true
		}
	}

	return "", "", false
}

func decodeGitPathLiteral(pathSpec string) (string, bool) {
	pathSpec = strings.TrimSpace(pathSpec)
	if pathSpec == "" {
		return "", false
	}

	if !strings.HasPrefix(pathSpec, "\"") {
		return pathSpec, true
	}

	// Git's quoted path format uses C-style escapes, which are compatible with
	// strconv.Unquote for the escape sequences Git emits in diff headers.
	decoded, err := strconv.Unquote(pathSpec)
	if err != nil {
		return "", false
	}
	return decoded, true
}

// needsGitQuoting returns true when a path contains characters that require
// C-style quoting in Git diff headers (spaces, control characters, backslashes,
// double-quotes, or non-ASCII bytes).
func needsGitQuoting(path string) bool {
	for _, b := range []byte(path) {
		if b == ' ' || b == '"' || b == '\\' || b < 0x20 || b >= 0x80 {
			return true
		}
	}
	return false
}

// DevPanelListBranches returns all branch names for a session's repository.
func (a *App) DevPanelListBranches(sessionName string) ([]string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}

	repoDir, err := a.resolveSessionRepoDir(sessionName)
	if err != nil {
		return nil, err
	}

	if !gitpkg.IsGitRepository(repoDir) {
		return nil, fmt.Errorf("not a git repository")
	}

	output, gitErr := gitpkg.RunGitCLIPublic(repoDir,
		[]string{"branch", "--format=%(refname:short)", "-a"})
	if gitErr != nil {
		return nil, fmt.Errorf("git branch failed: %w", gitErr)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}
