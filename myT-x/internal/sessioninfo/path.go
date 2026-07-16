package sessioninfo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

const DirName = "session-info"

// FolderKey returns the stable per-workdir directory key for session-scoped data.
func FolderKey(workDir string) (string, error) {
	normalized, err := normalizedWorkDir(workDir)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:]), nil
}

// FilePath returns a file path under <configDir>/session-info/<FolderKey(workDir)>.
func FilePath(configDir, workDir, fileName string) (string, error) {
	fileName, err := cleanFileName(fileName)
	if err != nil {
		return "", err
	}
	dir, err := DirectoryPath(configDir, workDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// DirectoryPath returns a directory path under <configDir>/session-info/<FolderKey(workDir)>.
func DirectoryPath(configDir, workDir string) (string, error) {
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return "", errors.New("config dir is empty")
	}
	absoluteConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	key, err := FolderKey(workDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Clean(absoluteConfigDir), DirName, key), nil
}

// LegacyProjectFilePath returns the old project-local path for session data.
func LegacyProjectFilePath(workDir, fileName string) (string, error) {
	fileName, err := cleanFileName(fileName)
	if err != nil {
		return "", err
	}
	normalized, err := normalizedWorkDir(workDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(normalized, ".myT-x", fileName), nil
}

func cleanFileName(fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "", errors.New("file name is empty")
	}
	if filepath.Base(fileName) != fileName ||
		strings.ContainsAny(fileName, `/\`) ||
		strings.Contains(fileName, "..") {
		return "", fmt.Errorf("file name %q must be a plain file name", fileName)
	}
	return fileName, nil
}

func normalizedWorkDir(workDir string) (string, error) {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return "", errors.New("workDir is empty")
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve workDir: %w", err)
	}
	cleaned := filepath.Clean(absolute)
	if realPath, err := filepath.EvalSymlinks(cleaned); err == nil {
		cleaned = filepath.Clean(realPath)
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned, nil
}
