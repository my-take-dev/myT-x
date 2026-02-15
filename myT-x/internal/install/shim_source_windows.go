//go:build windows

package install

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"myT-x/internal/procutil"
)

func findShimSource(workspaceRoot string) (string, error) {
	exePath, exeErr := os.Executable()
	if exeErr != nil {
		slog.Debug("[DEBUG-SHIM] os.Executable failed while locating shim source", "error", exeErr)
	}
	if exeErr == nil && exePath != "" {
		candidate := filepath.Join(filepath.Dir(exePath), "tmux-shim.exe")
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	if workspaceRoot != "" && fileExists(filepath.Join(workspaceRoot, "go.mod")) {
		target := filepath.Join(workspaceRoot, "tmux-shim.exe")
		// SECURITY: "go build" args are fixed constants; workspaceRoot is validated as a Go project via go.mod check above.
		cmd := exec.Command("go", "build", "-o", target, "./cmd/tmux-shim")
		procutil.HideWindow(cmd)
		cmd.Dir = workspaceRoot
		if output, err := cmd.CombinedOutput(); err == nil {
			return target, nil
		} else {
			return "", fmt.Errorf("build tmux-shim failed (dir=%s): %v (%s)", workspaceRoot, err, strings.TrimSpace(string(output)))
		}
	}

	return "", errors.New("tmux-shim.exe source not found")
}

func copyFile(src, dst string) (retErr error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Write to a temporary file in the same directory, then rename atomically.
	// This prevents a partial write from corrupting the existing destination.
	tmpFile, err := os.CreateTemp(filepath.Dir(dst), ".shim-*.tmp")
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
			if rErr := os.Remove(tmpPath); rErr != nil {
				slog.Debug("[DEBUG-SHIM] failed to remove temp file", "path", tmpPath, "error", rErr)
			}
		}
	}()

	if _, err := io.Copy(tmpFile, in); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
