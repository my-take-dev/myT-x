//go:build windows

package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var ensurePathContainsFn = ensurePathContains

// ShimInstallResult contains shim install details.
type ShimInstallResult struct {
	InstalledPath  string `json:"installed_path"`
	PathUpdated    bool   `json:"path_updated"`
	RestartNeeded  bool   `json:"restart_needed"`
	InstallMessage string `json:"message"`
}

// EnsureShimInstalled installs tmux shim and adds install dir to user PATH.
func EnsureShimInstalled(workspaceRoot string) (ShimInstallResult, error) {
	installDir, err := ResolveInstallDir()
	if err != nil {
		return ShimInstallResult{}, err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return ShimInstallResult{}, err
	}
	target := filepath.Join(installDir, "tmux.exe")
	hashFile := target + ".sha256"

	if HasEmbeddedShim() {
		shimBytes := GetEmbeddedShim()
		sourceHash := sha256Hex(shimBytes)
		if err := installShimIfChanged(hashFile, sourceHash, target, func() error {
			slog.Debug("[DEBUG-SHIM] writing embedded shim binary", "target", target, "size", len(shimBytes))
			return writeFileAtomically(target, shimBytes, 0o755)
		}); err != nil {
			return ShimInstallResult{}, fmt.Errorf("write embedded shim: %w", err)
		}
	} else {
		source, err := findShimSource(workspaceRoot)
		if err != nil {
			return ShimInstallResult{}, err
		}
		sourceHash, hashErr := sha256HexFile(source)
		if hashErr != nil {
			slog.Warn("[WARN-SHIM] hash computation failed, proceeding with copy", "error", hashErr)
			sourceHash = "" // force copy
		}
		if err := installShimIfChanged(hashFile, sourceHash, target, func() error {
			return copyFile(source, target)
		}); err != nil {
			return ShimInstallResult{}, err
		}
	}

	updated, err := ensurePathContainsFn(installDir)
	if err != nil {
		return ShimInstallResult{}, err
	}
	msg := "tmux shim installed"
	if updated {
		msg = "tmux shim installed; open a new terminal session to use updated PATH"
	}
	return ShimInstallResult{
		InstalledPath:  target,
		PathUpdated:    updated,
		RestartNeeded:  updated,
		InstallMessage: msg,
	}, nil
}

// NeedsShimInstall reports whether shim file or PATH registration is missing.
func NeedsShimInstall() (bool, error) {
	installDir, err := ResolveInstallDir()
	if err != nil {
		return false, err
	}

	target := filepath.Join(installDir, "tmux.exe")
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	regValue, err := readUserPathFromRegistry()
	if err != nil {
		return false, err
	}
	return !containsPathEntry(regValue, installDir), nil
}

// EnsureProcessPathContains adds dir to the current process PATH if absent.
// Child processes (terminal panes) inherit the updated PATH so the tmux shim
// becomes discoverable without requiring a system restart.
func EnsureProcessPathContains(dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false
	}
	// Keep process-level PATH updates serialized to avoid duplicate append races
	// between concurrent startup/background callers.
	ensurePathMu.Lock()
	defer ensurePathMu.Unlock()

	currentPath := os.Getenv("PATH")
	if containsPathEntry(currentPath, dir) {
		return false
	}
	if err := os.Setenv("PATH", currentPath+";"+dir); err != nil {
		slog.Warn("[WARN-SHIM] failed to update process PATH", "error", err)
		return false
	}
	return true
}

// ResolveInstallDir returns the shim installation directory path.
// ({LOCALAPPDATA}/myT-x/bin)
func ResolveInstallDir() (string, error) {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(localAppData, "myT-x", "bin"), nil
}

// sha256Hex returns the hex-encoded SHA256 hash of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// sha256HexFile returns the hex-encoded SHA256 hash of a file.
func sha256HexFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// installShimIfChanged compares sourceHash against the stored hash file and runs
// the writeFn only when the hashes differ. After a successful write, the hash
// file is updated. An empty sourceHash always triggers a write (hash unknown).
func installShimIfChanged(hashFile, sourceHash, target string, writeFn func() error) error {
	if sourceHash != "" && matchesHashFile(hashFile, sourceHash) {
		slog.Debug("[DEBUG-SHIM] shim unchanged, skipping write", "target", target)
		return nil
	}
	if err := writeFn(); err != nil {
		return err
	}
	if sourceHash != "" {
		if err := os.WriteFile(hashFile, []byte(sourceHash), 0o644); err != nil {
			slog.Warn("[WARN-SHIM] failed to write hash file", "path", hashFile, "error", err)
		}
	}
	return nil
}

func writeFileAtomically(target string, content []byte, perm os.FileMode) (retErr error) {
	tmpFile, err := os.CreateTemp(filepath.Dir(target), ".shim-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if retErr != nil {
			if closeErr := tmpFile.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				slog.Debug("[DEBUG-SHIM] failed to close temp file during rollback",
					"path", tmpPath, "error", closeErr)
			}
			if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
				slog.Debug("[DEBUG-SHIM] failed to remove temp file during rollback",
					"path", tmpPath, "error", removeErr)
			}
		}
	}()
	if _, err := tmpFile.Write(content); err != nil {
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}

// matchesHashFile reads a hash file and compares with the expected hash.
func matchesHashFile(hashFilePath, expectedHash string) bool {
	stored, err := os.ReadFile(hashFilePath)
	if err != nil {
		// NOTE: File not found is expected on first install; other IO errors
		// (permission, disk failure) are logged for diagnostics.
		if !os.IsNotExist(err) {
			slog.Debug("[DEBUG-SHIM] failed to read hash file", "path", hashFilePath, "error", err)
		}
		return false
	}
	return strings.TrimSpace(string(stored)) == strings.TrimSpace(expectedHash)
}
