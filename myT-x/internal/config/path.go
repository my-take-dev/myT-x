package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var windowsEnvTokenPattern = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)
var posixEnvTokenPattern = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*`)

var defaultPathWarningState struct {
	mu       sync.Mutex
	messages []string
}

func recordDefaultPathWarning(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	defaultPathWarningState.mu.Lock()
	defaultPathWarningState.messages = append(defaultPathWarningState.messages, trimmed)
	defaultPathWarningState.mu.Unlock()
}

// ConsumeDefaultPathWarnings returns and clears path-resolution warnings
// accumulated during DefaultPath() calls.
func ConsumeDefaultPathWarnings() []string {
	defaultPathWarningState.mu.Lock()
	defer defaultPathWarningState.mu.Unlock()
	if len(defaultPathWarningState.messages) == 0 {
		return nil
	}
	out := make([]string, len(defaultPathWarningState.messages))
	copy(out, defaultPathWarningState.messages)
	defaultPathWarningState.messages = nil
	return out
}

// DefaultPath resolves the config file path, preferring LOCALAPPDATA over
// APPDATA, falling back to ~/.config when both are unset, and then to
// os.TempDir() if the home directory cannot be resolved.
// The temp-dir fallback is not a stable persistence location and may vary
// between sessions depending on environment configuration.
func DefaultPath() string {
	return defaultPathWith(os.UserHomeDir)
}

// defaultPathWith is the parameterized implementation of DefaultPath,
// allowing tests to inject test doubles for os.UserHomeDir.
func defaultPathWith(userHomeDirFn func() (string, error)) string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("APPDATA"))
	}
	if base == "" {
		home, err := userHomeDirFn()
		if err != nil {
			// Keep config path resolvable even in restricted environments.
			slog.Warn("[WARN-CONFIG] using temp dir as config path fallback", "error", err)
			recordDefaultPathWarning(
				"Config path fallback: failed to resolve LOCALAPPDATA/APPDATA/home directory. Using temp directory; settings persistence may be limited.",
			)
			base = os.TempDir()
		} else {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "myT-x", "config.yaml")
}

func defaultConfigDir() (string, error) {
	return filepath.Dir(DefaultPath()), nil
}

// validateConfigPath normalizes path and enforces that config writes stay
// inside the default config directory when that directory is resolvable.
func validateConfigPath(path string) (string, error) {
	return validateConfigPathWith(defaultConfigDir, path)
}

// validateConfigPathWith is the parameterized implementation of validateConfigPath,
// allowing tests to inject test doubles for config directory resolution.
func validateConfigPathWith(configDirFn func() (string, error), path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("config path required")
	}
	absolutePath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return "", fmt.Errorf("save config: resolve path: %w", err)
	}

	expectedDir, err := configDirFn()
	if err != nil {
		return "", fmt.Errorf("save config: resolve config dir: %w", err)
	}
	absoluteExpectedDir, err := filepath.Abs(expectedDir)
	if err != nil {
		return "", fmt.Errorf("save config: resolve config dir: %w", err)
	}
	if !pathWithinDir(absolutePath, absoluteExpectedDir) {
		return "", fmt.Errorf("save config: path outside config directory: %q", absolutePath)
	}

	return absolutePath, nil
}

// pathWithinDir blocks directory traversal by ensuring path is under dir.
// It also rejects Windows cross-drive escapes because filepath.Rel returns
// an absolute path when roots differ.
func pathWithinDir(path string, dir string) bool {
	relativePath, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	if relativePath == "." {
		return true
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return false
	}
	return !filepath.IsAbs(relativePath)
}

// expandDefaultSessionDirEnv expands environment variable tokens in a directory path.
// Windows-style %VAR% tokens are expanded on all platforms.
// POSIX-style $VAR tokens are only expanded on non-Windows platforms to avoid
// misinterpreting '$' which is valid in Windows file paths.
func expandDefaultSessionDirEnv(dir string) string {
	if dir == "" {
		return ""
	}
	// Expand Windows-style %VAR% tokens on all platforms for portability.
	expanded := windowsEnvTokenPattern.ReplaceAllStringFunc(dir, func(token string) string {
		key := token[1 : len(token)-1]
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return token
	})
	// Skip POSIX-style $VAR expansion on Windows: '$' is a valid character
	// in Windows file paths (e.g. C:\Users\foo$bar) and should not be
	// interpreted as an environment variable reference.
	if runtime.GOOS == "windows" {
		return expanded
	}
	expanded = posixEnvTokenPattern.ReplaceAllStringFunc(expanded, func(token string) string {
		key := strings.TrimPrefix(token, "$")
		key = strings.TrimPrefix(key, "{")
		key = strings.TrimSuffix(key, "}")
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return token
	})
	return expanded
}
