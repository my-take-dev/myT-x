package sessionmemo

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"myT-x/internal/sessioninfo"
)

const (
	memoFileName = "session-memo.md"
	// 1 MiB keeps sidebar memo reads bounded while staying far above normal notes.
	maxMemoBytes = 1 << 20
)

// Deps contains App-level functions required by the session memo service.
type Deps struct {
	ResolveSessionWorkDir func(sessionName string) (string, error)
	ConfigDir             func() (string, error)
}

// Service loads, caches, and persists memo text for each terminal session.
type Service struct {
	deps Deps
	// fileIOMu serializes Windows file replacement for the memo file. It is
	// intentionally separate from mu so cache state is not locked during I/O.
	fileIOMu              sync.Mutex
	mu                    sync.Mutex
	memoByPath            map[string]string
	memoPathBySessionName map[string]string
	memoVersionByPath     map[string]uint64
	nextMemoVersion       uint64
}

// NewService creates a session memo service.
func NewService(deps Deps) *Service {
	if deps.ResolveSessionWorkDir == nil || deps.ConfigDir == nil {
		panic("sessionmemo.NewService: required function fields in Deps must be non-nil (ResolveSessionWorkDir, ConfigDir)")
	}
	return &Service{
		deps:                  deps,
		memoByPath:            make(map[string]string),
		memoPathBySessionName: make(map[string]string),
		memoVersionByPath:     make(map[string]uint64),
	}
}

// Load returns the current session memo from app-config session storage.
func (s *Service) Load(sessionName string) (string, error) {
	normalizedSessionName, path, legacyPath, err := s.resolveMemoPaths(sessionName)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	startVersion := s.memoVersionByPath[path]
	s.mu.Unlock()

	s.fileIOMu.Lock()
	if err := migrateLegacyMemoIfNeeded(path, legacyPath); err != nil {
		s.fileIOMu.Unlock()
		return "", fmt.Errorf("migrate legacy session memo: %w", err)
	}
	memo, err := readMemo(path)
	s.fileIOMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("read session memo: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.rememberSessionPathLocked(normalizedSessionName, path)
	if s.memoVersionByPath[path] != startVersion {
		if cachedMemo, ok := s.memoByPath[path]; ok {
			return cachedMemo, nil
		}
	}
	s.memoByPath[path] = memo
	return memo, nil
}

// Save writes the session memo to app-config session storage and updates the in-memory cache.
func (s *Service) Save(sessionName string, text string) error {
	normalizedSessionName, path, _, err := s.resolveMemoPaths(sessionName)
	if err != nil {
		return err
	}
	if len([]byte(text)) > maxMemoBytes {
		return fmt.Errorf("session memo must be %d bytes or fewer", maxMemoBytes)
	}

	s.fileIOMu.Lock()
	err = writeMemo(path, text)
	s.fileIOMu.Unlock()
	if err != nil {
		return fmt.Errorf("write session memo: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.rememberSessionPathLocked(normalizedSessionName, path)
	s.memoByPath[path] = text
	s.nextMemoVersion++
	s.memoVersionByPath[path] = s.nextMemoVersion
	return nil
}

// CleanupSession removes in-memory memo state associated with a destroyed session.
func (s *Service) CleanupSession(sessionName string) error {
	normalizedSessionName := strings.TrimSpace(sessionName)
	if normalizedSessionName == "" {
		return nil
	}

	s.mu.Lock()
	path := s.memoPathBySessionName[normalizedSessionName]
	if path != "" {
		s.forgetSessionPathLocked(normalizedSessionName, path)
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	_, resolvedPath, _, err := s.resolveMemoPaths(normalizedSessionName)
	if err != nil {
		slog.Debug("[DEBUG-SESSION-MEMO] cleanup skipped because memo path could not be resolved",
			"session", normalizedSessionName,
			"error", err,
		)
		return nil
	}

	s.mu.Lock()
	s.forgetSessionPathLocked(normalizedSessionName, resolvedPath)
	s.mu.Unlock()
	return nil
}

// RenameSession moves in-memory session-name indexing after a session rename.
func (s *Service) RenameSession(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if path := s.memoPathBySessionName[oldName]; path != "" {
		delete(s.memoPathBySessionName, oldName)
		s.memoPathBySessionName[newName] = path
	}
	return nil
}

func (s *Service) resolveMemoPaths(sessionName string) (string, string, string, error) {
	normalizedSessionName := strings.TrimSpace(sessionName)
	if normalizedSessionName == "" {
		return "", "", "", errors.New("session name is required for session memo")
	}

	workDir, err := s.deps.ResolveSessionWorkDir(normalizedSessionName)
	if err != nil {
		return "", "", "", err
	}
	configDir, err := s.deps.ConfigDir()
	if err != nil {
		return "", "", "", err
	}
	path, err := sessioninfo.FilePath(configDir, workDir, memoFileName)
	if err != nil {
		return "", "", "", err
	}
	legacyPath, err := sessioninfo.LegacyProjectFilePath(workDir, memoFileName)
	if err != nil {
		return "", "", "", err
	}
	return normalizedSessionName, path, legacyPath, nil
}

func (s *Service) rememberSessionPathLocked(sessionName, path string) {
	s.memoPathBySessionName[sessionName] = path
}

func (s *Service) forgetSessionPathLocked(sessionName, path string) {
	delete(s.memoPathBySessionName, sessionName)
	delete(s.memoByPath, path)
	delete(s.memoVersionByPath, path)
}

func readMemo(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.Size() > maxMemoBytes {
		return "", fmt.Errorf("session memo file is too large: %d bytes", info.Size())
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Debug("[DEBUG-SESSION-MEMO] failed to close memo file", "path", path, "error", closeErr)
		}
	}()

	data, err := io.ReadAll(io.LimitReader(file, maxMemoBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxMemoBytes {
		return "", fmt.Errorf("session memo file is too large: %d bytes", len(data))
	}
	return string(data), nil
}

func migrateLegacyMemoIfNeeded(currentPath, legacyPath string) error {
	if _, err := os.Stat(currentPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	memo, err := readMemo(legacyPath)
	if err != nil {
		return err
	}
	if err := writeMemo(currentPath, memo); err != nil {
		return err
	}
	slog.Info("[DEBUG-SESSION-MEMO] migrated legacy project memo to session-info",
		"legacy_path", legacyPath,
		"current_path", currentPath,
	)
	return nil
}

func writeMemo(path string, memo string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session memo directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".session-memo.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for session memo: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpFile != nil {
			// Successful close transfers ownership to the filesystem; avoid a second
			// close while still closing failed-write handles.
			if closeErr := tmpFile.Close(); closeErr != nil {
				slog.Debug("[DEBUG-SESSION-MEMO] failed to close temp file", "path", tmpPath, "error", closeErr)
			}
		}
		// Rename removes tmpPath on success. If it still exists, the write failed
		// before replacement and the leftover temp file can be safely removed.
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			if removeErr := os.Remove(tmpPath); removeErr != nil {
				slog.Debug("[DEBUG-SESSION-MEMO] failed to remove temp file", "path", tmpPath, "error", removeErr)
			}
		} else if !os.IsNotExist(statErr) {
			slog.Debug("[DEBUG-SESSION-MEMO] failed to stat temp file", "path", tmpPath, "error", statErr)
		}
	}()

	if _, err := tmpFile.WriteString(memo); err != nil {
		return fmt.Errorf("write session memo temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync session memo temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close session memo temp file: %w", err)
	}
	tmpFile = nil

	if err := replaceMemoFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace session memo file: %w", err)
	}
	return nil
}
