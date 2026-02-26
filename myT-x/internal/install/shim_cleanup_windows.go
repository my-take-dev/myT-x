//go:build windows

package install

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// legacyInstallSubPaths lists known legacy shim installation subdirectories
// under %LOCALAPPDATA% that should be cleaned up on startup.
var legacyInstallSubPaths = []string{
	filepath.Join("github.com", "my-take-dev", "myT-x"),
}

// stalePathSubstring matches PATH entries injected by TestEnsureShim integration tests.
// This value must match the temp-dir pattern used in shim_cleanup_windows_test.go.
const stalePathSubstring = `\Temp\TestEnsureShim`

// CleanupLegacyShimInstalls removes legacy shim directories and their
// PATH entries from the Windows user PATH registry. This function is
// idempotent and safe to call on every startup. Errors are logged but
// never block startup.
// Always returns nil: errors are logged internally, not propagated.
// The error return is retained for interface compatibility with the
// function variable in app_lifecycle.go and the non-Windows stub.
func CleanupLegacyShimInstalls() error {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		slog.Debug("[DEBUG-SHIM] LOCALAPPDATA empty, skipping legacy cleanup")
		return nil
	}

	for _, sub := range legacyInstallSubPaths {
		legacyBase := filepath.Join(localAppData, sub)
		legacyBinDir := filepath.Join(legacyBase, "bin")

		removed := removeLegacyPathEntry(legacyBinDir)
		if removed {
			slog.Info("[shim] removed legacy PATH entry", "dir", legacyBinDir)
		}

		removeLegacyDirectory(legacyBase, localAppData)

		removeProcessPathEntry(legacyBinDir)
	}

	cleanupStalePathEntries()
	return nil
}

// removeLegacyPathEntry removes a specific directory entry from the Windows
// user PATH in the registry. Returns true if the PATH was modified.
func removeLegacyPathEntry(legacyDir string) bool {
	ensurePathMu.Lock()
	defer ensurePathMu.Unlock()

	// OpenKey (not CreateKey): this is a read/update-only operation.
	// If the registry key does not exist, there is no PATH to clean up.
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		registryEnvKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			slog.Debug("[DEBUG-SHIM] legacy cleanup: registry key does not exist, nothing to clean")
			return false
		}
		slog.Warn("[WARN-SHIM] legacy cleanup: failed to open registry key", "error", err)
		return false
	}
	defer key.Close()

	regValue, regValueType, err := readUserPathFromRegistryKeyWithType(key)
	if err != nil {
		slog.Warn("[WARN-SHIM] legacy cleanup: failed to read PATH", "error", err)
		return false
	}

	if !containsPathEntry(regValue, legacyDir) {
		return false
	}

	newPath := filterPathEntries(regValue, legacyDir)

	targetValueType := selectPathRegistryValueType(regValueType, newPath)
	if err := setPathRegistryValue(key, newPath, targetValueType); err != nil {
		slog.Warn("[WARN-SHIM] legacy cleanup: failed to write PATH", "error", err)
		return false
	}

	if err := broadcastEnvironmentSettingChange(); err != nil {
		slog.Warn("[WARN-SHIM] legacy cleanup: broadcast failed", "error", err)
	}
	return true
}

// filterPathEntries removes all entries matching removeDir from a
// semicolon-separated PATH string. Comparison is case-insensitive
// using filepath.Clean, matching containsPathEntry behavior.
func filterPathEntries(pathValue string, removeDir string) string {
	normalizedRemove := strings.ToLower(filepath.Clean(strings.TrimSpace(removeDir)))

	// Guard: filepath.Clean("") returns "." which would unintentionally
	// strip current-directory entries from PATH. Return unchanged.
	if normalizedRemove == "." {
		return pathValue
	}

	var kept []string
	for item := range strings.SplitSeq(pathValue, ";") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if strings.ToLower(filepath.Clean(trimmed)) == normalizedRemove {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, ";")
}

// removeLegacyDirectory removes known shim-related files from the legacy
// base directory and then removes empty parent directories up to LOCALAPPDATA.
// localAppData must be a non-empty, already-validated LOCALAPPDATA path.
func removeLegacyDirectory(legacyBase string, localAppData string) {
	binDir := filepath.Join(legacyBase, "bin")

	// Remove known files in bin/
	knownBinFiles := []string{"tmux.exe", "tmux.exe.sha256"}
	for _, name := range knownBinFiles {
		removeFileIfExists(filepath.Join(binDir, name))
	}
	removeEmptyDir(binDir)

	// Remove known files in base dir
	knownBaseFiles := []string{"config.yaml", "shim-debug.log"}
	for _, name := range knownBaseFiles {
		removeFileIfExists(filepath.Join(legacyBase, name))
	}

	// Remove rotated shim-debug-*.log files
	matches, err := filepath.Glob(filepath.Join(legacyBase, "shim-debug-*.log"))
	if err == nil {
		for _, m := range matches {
			removeFileIfExists(m)
		}
	}

	// Walk up parent directories removing empty ones, stopping at LOCALAPPDATA.
	// Clean both paths to normalize trailing separators and redundant elements,
	// ensuring consistent comparison and correct os.Remove / filepath.Dir behavior.
	normalizedLocalAppData := strings.ToLower(filepath.Clean(localAppData))
	current := filepath.Clean(legacyBase)
	for {
		normalizedCurrent := strings.ToLower(filepath.Clean(current))
		if normalizedCurrent == normalizedLocalAppData {
			break
		}
		// Reached filesystem root; stop walk-up.
		if current == filepath.Dir(current) {
			break
		}
		if err := os.Remove(current); err != nil {
			break // Not empty or permission error; stop walking up.
		}
		slog.Debug("[DEBUG-SHIM] removed empty legacy dir", "dir", current)
		current = filepath.Dir(current)
	}
}

// cleanupStalePathEntries removes PATH entries that match known junk
// patterns (e.g. test temp directories) from the Windows user PATH.
func cleanupStalePathEntries() {
	ensurePathMu.Lock()
	defer ensurePathMu.Unlock()

	// OpenKey (not CreateKey): this is a read/update-only operation.
	// If the registry key does not exist, there are no stale entries to clean.
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		registryEnvKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			slog.Debug("[DEBUG-SHIM] stale cleanup: registry key does not exist, nothing to clean")
			return
		}
		slog.Warn("[WARN-SHIM] stale cleanup: failed to open registry key", "error", err)
		return
	}
	defer key.Close()

	regValue, regValueType, err := readUserPathFromRegistryKeyWithType(key)
	if err != nil {
		slog.Warn("[WARN-SHIM] stale cleanup: failed to read PATH", "error", err)
		return
	}

	newPath, removedCount := filterStalePathEntries(regValue, stalePathSubstring)

	if removedCount == 0 {
		return
	}
	targetValueType := selectPathRegistryValueType(regValueType, newPath)
	if err := setPathRegistryValue(key, newPath, targetValueType); err != nil {
		slog.Warn("[WARN-SHIM] stale cleanup: failed to write PATH", "error", err)
		return
	}

	if err := broadcastEnvironmentSettingChange(); err != nil {
		slog.Warn("[WARN-SHIM] stale cleanup: broadcast failed", "error", err)
	}
	slog.Info("[shim] removed stale PATH entries", "count", removedCount)
}

// filterStalePathEntries removes PATH entries whose lowercase representation
// contains the given substring (case-insensitive match). Returns the filtered
// PATH string and the number of removed entries.
func filterStalePathEntries(pathValue string, substring string) (string, int) {
	if substring == "" {
		return pathValue, 0
	}
	staleLower := strings.ToLower(substring)
	var kept []string
	removedCount := 0
	for item := range strings.SplitSeq(pathValue, ";") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), staleLower) {
			removedCount++
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, ";"), removedCount
}

// removeProcessPathEntry removes dir from the current process PATH
// environment variable. Returns true if the PATH was modified.
func removeProcessPathEntry(dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false
	}
	ensurePathMu.Lock()
	defer ensurePathMu.Unlock()

	currentPath := os.Getenv("PATH")
	if !containsPathEntry(currentPath, dir) {
		return false
	}

	newPath := filterPathEntries(currentPath, dir)
	if err := os.Setenv("PATH", newPath); err != nil {
		slog.Warn("[WARN-SHIM] failed to update process PATH for legacy removal", "error", err)
		return false
	}
	return true
}

func removeFileIfExists(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Debug("[DEBUG-SHIM] failed to remove legacy file", "path", path, "error", err)
	}
}

func removeEmptyDir(dir string) {
	if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Debug("[DEBUG-SHIM] failed to remove legacy dir", "dir", dir, "error", err)
	}
}
