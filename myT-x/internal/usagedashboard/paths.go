package usagedashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AbsPathToClaudeSlug converts a Windows absolute path to a Claude Code
// projects/ slug. Every character that is not an ASCII letter, digit, or
// hyphen is replaced with a single hyphen. This matches the slug format
// Claude Code uses under ~/.claude/projects/.
//
//	D:\myT-x\dev-myT-x             → D--myT-x-dev-myT-x
//	C:\Users\x\y                   → C--Users-x-y
//	D:\test_repository\test_repo   → D--test-repository-test-repo
//	C:\Users\mytakedev\.claude-mem → C--Users-mytakedev--claude-mem
//
// The function is deterministic and does not hit the filesystem. For reverse
// lookup that validates against the actual projects/ directory, see
// FindClaudeProjectDir.
func AbsPathToClaudeSlug(absPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(absPath))
	if cleaned == "" || cleaned == "." {
		return ""
	}
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		if isSlugChar(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func isSlugChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '-':
		return true
	default:
		return false
	}
}

// ResolveClaudeHome returns the .claude directory path under the provided home.
// If homeDir is empty, falls back to os.UserHomeDir.
func ResolveClaudeHome(homeDir string) (string, error) {
	if strings.TrimSpace(homeDir) == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		homeDir = h
	}
	return filepath.Join(homeDir, ".claude"), nil
}

// ResolveCodexHome returns the .codex directory path under the provided home.
func ResolveCodexHome(homeDir string) (string, error) {
	if strings.TrimSpace(homeDir) == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		homeDir = h
	}
	return filepath.Join(homeDir, ".codex"), nil
}

// FindClaudeProjectDir locates the Claude projects/<slug> directory for absPath.
// Returns the absolute slug directory path or an empty string with a non-nil
// error when the slug directory does not exist.
//
// The function performs an os.ReadDir on ~/.claude/projects and matches the
// computed slug case-sensitively. If no match is found, it also attempts a
// case-insensitive fallback to tolerate drive-letter and path-segment case
// differences observed on Windows.
func FindClaudeProjectDir(claudeHome, absPath string) (string, error) {
	slug := AbsPathToClaudeSlug(absPath)
	if slug == "" {
		return "", fmt.Errorf("empty absolute path")
	}
	projectsDir := filepath.Join(claudeHome, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", fmt.Errorf("read claude projects dir %s: %w", projectsDir, err)
	}
	// Exact match first.
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == slug {
			return filepath.Join(projectsDir, entry.Name()), nil
		}
	}
	// Case-insensitive fallback (Windows drive letter / path segment case drift).
	for _, entry := range entries {
		if entry.IsDir() && strings.EqualFold(entry.Name(), slug) {
			return filepath.Join(projectsDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("claude project dir not found for %s (slug: %s)", absPath, slug)
}

// PathsEqualFold reports whether two filesystem paths refer to the same
// location after normalization. Windows paths are compared case-insensitively.
// Empty inputs are treated as non-matching to avoid "" == "." collisions from
// filepath.Clean.
func PathsEqualFold(a, b string) bool {
	ta := strings.TrimSpace(a)
	tb := strings.TrimSpace(b)
	if ta == "" || tb == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(ta), filepath.Clean(tb))
}
