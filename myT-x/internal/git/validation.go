package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// branchNameRegex validates git branch names.
// Allowed characters: alphanumeric, dots, underscores, hyphens, and slashes.
var branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

// customNameRegex matches characters that are NOT allowed in custom names.
// Allowed characters: alphanumeric, hyphens, and underscores.
var customNameRegex = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

const wtDirSuffix = ".wt"

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
	// filepath.Clean normalizes the path and also catches raw ".." sequences.
	if strings.Contains(filepath.Clean(name), "..") {
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

// SanitizeCustomName removes invalid characters from custom name.
// Allowed characters: [a-zA-Z0-9-_]. Converts to lowercase, returns "work" as default if empty.
func SanitizeCustomName(name string) string {
	sanitized := customNameRegex.ReplaceAllString(strings.ToLower(name), "")
	if sanitized == "" {
		return "work"
	}
	return sanitized
}

// GenerateBranchName creates a unique branch name in the format: {baseBranch}-{customName}-{UnixNano}.
func GenerateBranchName(baseBranch, customName string) string {
	sanitizedName := SanitizeCustomName(customName)
	uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
	return fmt.Sprintf("%s-%s-%s", baseBranch, sanitizedName, uniqueID)
}

// GenerateWorktreeDirPath returns the .wt directory path for a repository.
// Given repoPath=/path/to/myapp, returns /path/to/myapp.wt
func GenerateWorktreeDirPath(repoPath string) string {
	return filepath.Join(filepath.Dir(repoPath), filepath.Base(repoPath)+wtDirSuffix)
}

// GenerateWorktreePath returns the full path for a specific worktree.
// Given repoPath=/path/to/myapp and identifier=feature-auth, returns /path/to/myapp.wt/feature-auth
func GenerateWorktreePath(repoPath, identifier string) string {
	return filepath.Join(GenerateWorktreeDirPath(repoPath), identifier)
}

// FindAvailableWorktreePath returns basePath if it does not exist.
// If basePath already exists, it appends -2, -3, ... until a free path is found.
func FindAvailableWorktreePath(basePath string) string {
	if _, err := os.Stat(basePath); err != nil {
		if os.IsNotExist(err) {
			return basePath
		}
		// NOTE: Permission errors or other I/O failures are logged but treated
		// as "path exists" to avoid accidentally reusing the path.
		slog.Debug("[DEBUG-GIT] FindAvailableWorktreePath: stat returned non-NotExist error",
			"path", basePath, "error", err)
	}
	for i := 2; i <= 100; i++ {
		candidate := fmt.Sprintf("%s-%d", basePath, i)
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
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
	if !filepath.IsAbs(path) {
		return fmt.Errorf("worktree path must be absolute: %s", path)
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("worktree path must not contain '..': %s", path)
	}
	base := filepath.Base(path)
	if base == ".git" || base == ".hg" || base == ".svn" {
		return fmt.Errorf("worktree path must not target VCS directory: %s", path)
	}
	return nil
}
