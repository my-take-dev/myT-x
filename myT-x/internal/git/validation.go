package git

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// branchNameRegex validates git branch names.
// Allowed characters: alphanumeric, dots, underscores, hyphens, and slashes.
var branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

// customNameRegex matches characters that are NOT allowed in custom names.
// Allowed characters: alphanumeric, hyphens, and underscores.
var customNameRegex = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

// commitishRegex validates commit-ish references accepted by worktree creation.
// Allowed characters are restricted to common ref/hash syntax to block control
// characters and whitespace while still allowing refs such as "HEAD~1".
var commitishRegex = regexp.MustCompile(`^[a-zA-Z0-9._/@^~:-]+$`)

const (
	// WtDirSuffix is the directory suffix for worktree directories (e.g. myapp.wt).
	WtDirSuffix           = ".wt"
	maxWorktreePathSuffix = 100
	warnWorktreePathLen   = 240
	maxWorktreePathLen    = 259
)

var windowsReservedPathNames = map[string]struct{}{
	"CON":  {},
	"PRN":  {},
	"AUX":  {},
	"NUL":  {},
	"COM1": {},
	"COM2": {},
	"COM3": {},
	"COM4": {},
	"COM5": {},
	"COM6": {},
	"COM7": {},
	"COM8": {},
	"COM9": {},
	"LPT1": {},
	"LPT2": {},
	"LPT3": {},
	"LPT4": {},
	"LPT5": {},
	"LPT6": {},
	"LPT7": {},
	"LPT8": {},
	"LPT9": {},
}

// IsValidBranchName checks if the given branch name is valid.
func IsValidBranchName(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") {
		return false
	}
	if strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return false
	}
	// Reject raw ".." sequences directly so names like "a/../b" are blocked.
	if strings.Contains(name, "..") {
		return false
	}
	if strings.Contains(name, "//") {
		return false
	}
	if strings.HasSuffix(name, ".lock") {
		return false
	}
	return branchNameRegex.MatchString(name)
}

// ValidateBranchName validates that a branch name is safe for git commands.
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if !IsValidBranchName(name) {
		return fmt.Errorf("invalid branch name: %s (must contain only alphanumeric characters, dots, underscores, hyphens, and slashes; cannot start with '.', '-', or '/')", name)
	}
	return nil
}

// ValidateCommitish validates a git commit-ish used as a worktree base.
func ValidateCommitish(commitish string) error {
	if strings.TrimSpace(commitish) == "" {
		return fmt.Errorf("commit-ish cannot be empty")
	}
	if strings.ContainsRune(commitish, '\x00') {
		return fmt.Errorf("invalid commit-ish: contains null byte")
	}
	if !commitishRegex.MatchString(commitish) {
		return fmt.Errorf("invalid commit-ish %q (allowed pattern: %s)", commitish, commitishRegex.String())
	}
	return nil
}

// structureSeparatorReplacer converts slashes, backslashes, and dots to hyphens.
// These characters carry structural information (e.g. feature/auth → feature-auth)
// that would be lost if simply stripped.
var structureSeparatorReplacer = strings.NewReplacer("/", "-", "\\", "-", ".", "-")

// consecutiveHyphenRegex matches two or more consecutive hyphens for collapsing.
var consecutiveHyphenRegex = regexp.MustCompile(`-{2,}`)

// SanitizeCustomName removes invalid characters from custom name.
// Slashes, backslashes, and dots are converted to hyphens to preserve structural
// information (e.g. "feature/user-auth" → "feature-user-auth").
// Remaining non-allowed characters are stripped. Consecutive hyphens are collapsed,
// leading/trailing hyphens are trimmed. Returns "work" as default if empty.
func SanitizeCustomName(name string) string {
	replaced := structureSeparatorReplacer.Replace(strings.ToLower(name))
	sanitized := customNameRegex.ReplaceAllString(replaced, "")
	sanitized = consecutiveHyphenRegex.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "work"
	}
	return sanitized
}

// GenerateWorktreeDirPath returns the .wt directory path for a repository.
// Given repoPath=/path/to/myapp, returns /path/to/myapp.wt
func GenerateWorktreeDirPath(repoPath string) string {
	return filepath.Join(filepath.Dir(repoPath), filepath.Base(repoPath)+WtDirSuffix)
}

// GenerateWorktreePath returns the full path for a specific worktree.
// Given repoPath=/path/to/myapp and identifier=feature-auth, returns /path/to/myapp.wt/feature-auth
func GenerateWorktreePath(repoPath, identifier string) string {
	return filepath.Join(GenerateWorktreeDirPath(repoPath), identifier)
}

// FindAvailableWorktreePath returns basePath if it does not exist.
// If basePath already exists, it appends -2, -3, ... until a free path is found.
//
// NOTE: TOCTOU — There is a time-of-check-to-time-of-use race between the
// os.Stat check here and the subsequent git worktree add. This is acceptable
// because git worktree add atomically acquires the path; if a concurrent
// creation takes the same path, the git command fails and the caller's rollback
// logic cleans up. The cost is at most one failed creation attempt.
func FindAvailableWorktreePath(basePath string) string {
	if _, err := os.Stat(basePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return basePath
		}
		// NOTE: Permission errors or other I/O failures are logged but treated
		// as "path exists" to avoid accidentally reusing the path.
		slog.Debug("[DEBUG-GIT] FindAvailableWorktreePath: stat returned non-NotExist error",
			"path", basePath, "error", err)
	}
	for i := 2; i <= maxWorktreePathSuffix; i++ {
		candidate := fmt.Sprintf("%s-%d", basePath, i)
		if _, err := os.Stat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return candidate
			}
			slog.Debug("[DEBUG-GIT] FindAvailableWorktreePath: stat returned non-NotExist error",
				"path", candidate, "error", err)
		}
	}
	// Fallback: use timestamp suffix.
	return fmt.Sprintf("%s-%d", basePath, time.Now().UnixMilli())
}

// ValidateWorktreePath validates that a worktree path is safe.
func ValidateWorktreePath(path string) error {
	if path == "" {
		return fmt.Errorf("worktree path cannot be empty")
	}
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("worktree path must not contain null bytes")
	}
	cleanedPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanedPath) {
		return fmt.Errorf("worktree path must be absolute: %s", path)
	}
	if isUNCPath(cleanedPath) {
		return fmt.Errorf("worktree path must not use UNC path: %s", path)
	}
	pathLen := utf8.RuneCountInString(cleanedPath)
	if pathLen > maxWorktreePathLen {
		return fmt.Errorf("worktree path exceeds Windows MAX_PATH limit (%d): %s", maxWorktreePathLen, path)
	}
	if pathLen > warnWorktreePathLen {
		slog.Warn("[WARN-GIT] worktree path is approaching Windows MAX_PATH limit",
			"path", cleanedPath, "length", pathLen, "limit", maxWorktreePathLen)
	}

	segments := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if slices.Contains(segments, "..") {
		return fmt.Errorf("worktree path must not contain '..' path segment: %s", path)
	}
	for _, segment := range trimVolumePrefixFromSegments(cleanedPath, segments) {
		if segment == "" {
			continue
		}
		if strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return fmt.Errorf("worktree path segment must not end with space or dot: %s", segment)
		}
		if isWindowsReservedPathName(segment) {
			return fmt.Errorf("worktree path segment uses reserved Windows device name: %s", segment)
		}
	}

	base := filepath.Base(cleanedPath)
	if base == ".git" || base == ".hg" || base == ".svn" {
		return fmt.Errorf("worktree path must not target VCS directory: %s", path)
	}
	return nil
}

func isUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) ||
		strings.HasPrefix(strings.ToLower(path), `\\?\`)
}

func trimVolumePrefixFromSegments(cleanedPath string, segments []string) []string {
	volume := filepath.VolumeName(cleanedPath)
	if volume == "" || len(segments) == 0 {
		return segments
	}
	if strings.EqualFold(segments[0], volume) {
		return segments[1:]
	}
	return segments
}

func isWindowsReservedPathName(segment string) bool {
	trimmed := strings.TrimSpace(segment)
	if trimmed == "" {
		return false
	}
	base := trimmed
	if dot := strings.IndexRune(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	_, ok := windowsReservedPathNames[strings.ToUpper(base)]
	return ok
}

// PostRemovalCleanup runs standard post-worktree-removal housekeeping:
// pruning stale git worktree entries and removing the empty .wt parent directory.
// Failures are logged but never propagated. Callers should handle branch
// cleanup separately as it is context-specific.
func PostRemovalCleanup(repo *Repository, wtPath string) {
	if repo != nil {
		if err := repo.PruneWorktrees(); err != nil {
			slog.Warn("[WARN-GIT] failed to prune worktrees after cleanup", "error", err)
		}
	}
	RemoveEmptyWtDir(wtPath)
}

// RemoveEmptyWtDir removes the .wt parent directory of wtPath if it is empty
// after worktree removal. Failures are logged but never propagated.
// Safe to call from any package that performs worktree cleanup.
func RemoveEmptyWtDir(wtPath string) {
	if wtPath == "" {
		return
	}
	wtDir := filepath.Dir(wtPath)
	if !strings.HasSuffix(wtDir, WtDirSuffix) {
		return
	}
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("[WARN-GIT] failed to read .wt directory for cleanup",
				"path", wtDir, "error", err)
		}
		return
	}
	if len(entries) == 0 {
		if err := os.Remove(wtDir); err != nil {
			slog.Debug("[DEBUG-GIT] failed to remove empty .wt directory",
				"path", wtDir, "error", err)
		}
	}
}
