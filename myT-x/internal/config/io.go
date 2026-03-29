package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	maxConfigFileBytes int64 = 1 << 20 // 1MB
	maxRenameRetry           = 10
	// Windows file lock releases (antivirus/indexing) typically settle quickly.
	// Use a short linear backoff: baseDelay * (1..maxRenameRetry).
	renameRetryBaseDelay = 10 * time.Millisecond
)

// Load reads config file. If file does not exist, defaults are returned.
// The configured shell is validated against an allowlist; an error is returned
// if validation fails.
func Load(path string) (Config, error) {
	return loadWith(parseRawConfigMetadata, path)
}

// loadWith is the parameterized implementation of Load,
// allowing tests to inject test doubles for the metadata parser.
func loadWith(metadataParserFn func([]byte) (map[string]any, error), path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, errors.New("config path required")
	}

	raw, err := readLimitedFile(path, maxConfigFileBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		slog.Warn("[WARN-CONFIG] failed to parse config, using defaults", "path", path, "error", err)
		return DefaultConfig(), err
	}

	rawMap, metadataErr := metadataParserFn(raw)
	defaultWorktreeEnabled := DefaultConfig().Worktree.Enabled
	if metadataErr != nil {
		slog.Warn("[WARN-CONFIG] failed to parse config metadata", "error", metadataErr)
	} else {
		// Warn about deprecated fields that are silently ignored by yaml.Unmarshal.
		warnDeprecatedFields(rawMap)
	}
	hasWorktreeEnabled, resolveErr := resolveWorktreeEnabled(raw, rawMap)
	if resolveErr != nil {
		// Keep already-parsed cfg.Worktree.Enabled to avoid silently overwriting
		// explicit user values when helper probing is unavailable.
		slog.Warn("[WARN-CONFIG] failed to resolve worktree.enabled metadata, preserving parsed value", "error", resolveErr)
	} else if !hasWorktreeEnabled {
		cfg.Worktree.Enabled = defaultWorktreeEnabled
	}
	if err := applyDefaultsAndValidate(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// EnsureFile writes default config if missing and returns loaded config.
func EnsureFile(path string) (Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return cfg, err
	}
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		if _, err := Save(path, cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// Save validates cfg, fills defaults, and atomically writes to path.
// Returns the normalized config that was actually written to disk.
// Uses the same validation rules as Load (shell allowlist, agent model constraints).
func Save(path string, cfg Config) (Config, error) {
	normalizedPath, err := validateConfigPath(path)
	if err != nil {
		return cfg, err
	}
	if err := applyDefaultsAndValidate(&cfg); err != nil {
		return cfg, fmt.Errorf("save config: %w", err)
	}

	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, fmt.Errorf("save config: marshal: %w", err)
	}
	if err := atomicWrite(normalizedPath, raw); err != nil {
		return cfg, err
	}
	slog.Debug("[DEBUG-CONFIG] config saved", "path", path)
	return cfg, nil
}

// atomicWrite writes config data using temp-file + rename to avoid partial
// writes and retries rename on Windows to tolerate transient file locks.
func atomicWrite(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save config: mkdir: %w", err)
	}

	// Atomic write: temp file + rename in same directory ensures
	// same-filesystem rename and prevents partial writes on crash.
	tmpFile, err := os.CreateTemp(dir, ".config.yaml.tmp.*")
	if err != nil {
		return fmt.Errorf("save config: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		if tmpFile != nil {
			if closeErr := tmpFile.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				slog.Warn("[WARN-CONFIG] failed to close temp file", "path", tmpPath, "error", closeErr)
			}
		}
		if err != nil {
			if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				slog.Warn("[WARN-CONFIG] failed to remove temp file", "path", tmpPath, "error", removeErr)
			}
		}
	}()

	if err = tmpFile.Chmod(0o600); err != nil {
		return fmt.Errorf("save config: chmod temp: %w", err)
	}
	if _, err = tmpFile.Write(data); err != nil {
		return fmt.Errorf("save config: write: %w", err)
	}
	if err = tmpFile.Sync(); err != nil {
		return fmt.Errorf("save config: sync: %w", err)
	}
	err = tmpFile.Close()
	tmpFile = nil
	if err != nil {
		return fmt.Errorf("save config: close: %w", err)
	}

	if err = renameFileWithRetry(tmpPath, path); err != nil {
		return fmt.Errorf("save config: rename: %w", err)
	}
	return nil
}

func readLimitedFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	limited := io.LimitReader(file, maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("config file exceeds %d bytes", maxBytes)
	}
	return raw, nil
}

func renameFileWithRetry(sourcePath string, targetPath string) error {
	var lastErr error
	for attempt := range maxRenameRetry {
		err := os.Rename(sourcePath, targetPath)
		if err == nil {
			return nil
		}
		lastErr = err
		if runtime.GOOS != "windows" {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * renameRetryBaseDelay)
	}
	return lastErr
}
