package devpanel

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"myT-x/internal/apptypes"
	gitpkg "myT-x/internal/git"
)

// maxFileSize is the maximum file size returned by ReadFile (1 MB).
const maxFileSize int64 = 1 << 20

const (
	retryBaseDelay = 10 * time.Millisecond
	maxRetries     = 5
)

// Windows file errors that are often transient because of antivirus/indexer locks.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
)

// maxDirEntries is the maximum number of directory entries returned by ListDir.
const maxDirEntries = 5000

// binaryProbeSize is the number of bytes scanned to detect binary content.
const binaryProbeSize = 8192

// maxDiffSize is the maximum diff output size (500 KB).
const maxDiffSize = 500 * 1024

// maxUntrackedFilePaths is the maximum number of untracked paths parsed
// from git ls-files output to prevent unbounded memory growth.
const maxUntrackedFilePaths = 10000

// maxGitLogCount is the maximum number of commits returned by GitLog.
const maxGitLogCount = 1000

// gitLogFieldCount is the expected number of NUL-delimited fields in git log
// format: full_hash, short_hash, parents, subject, author_name, author_date_iso, decorations.
const gitLogFieldCount = 7

// maxUntrackedFileSize is the maximum size of an individual untracked file
// included in the working diff (100 KB). Larger files are skipped to avoid memory bloat.
const maxUntrackedFileSize int64 = 100 * 1024

// DetachedHEADSentinel is the branch name returned by GitStatus when the
// repository is in detached HEAD state. The frontend uses this value to
// distinguish detached HEAD from an error (where Branch remains "").
const DetachedHEADSentinel = "(HEAD detached)"

// excludedDirs contains directory names excluded from listings.
var excludedDirs = []string{".git", "node_modules"}

// Deps holds external dependencies injected into the devpanel Service.
type Deps struct {
	// ResolveSessionDir resolves a directory path for a session.
	// When preferWorktree is true, returns the worktree path (working directory).
	// When preferWorktree is false, returns the repo path (git operations).
	ResolveSessionDir func(sessionName string, preferWorktree bool) (string, error)

	// IsPathWithinBase checks whether path is contained within base directory.
	// Both path and base must be resolved (e.g. via filepath.EvalSymlinks)
	// before calling; passing unresolved paths may produce incorrect results.
	IsPathWithinBase func(path, base string) bool

	// Emitter broadcasts frontend runtime events.
	// Defaults to a no-op emitter when nil.
	Emitter apptypes.RuntimeEventEmitter
}

// Service provides developer panel file/directory browsing and git operations.
// External dependencies are injected via Deps; short-lived directory cache and
// watcher lifecycle state are owned internally to reduce repeated filesystem I/O.
type Service struct {
	deps           Deps
	dirCache       *DirCache
	watcherManager *watcherManager
}

// NewService creates a new devpanel Service with the given dependencies.
// Panics if required dependency fields are nil.
func NewService(deps Deps) *Service {
	if deps.ResolveSessionDir == nil || deps.IsPathWithinBase == nil {
		panic("devpanel.NewService: required function fields must be non-nil (ResolveSessionDir, IsPathWithinBase)")
	}
	if deps.Emitter == nil {
		deps.Emitter = apptypes.NoopEmitter{}
	}

	dirCache := NewDirCache(defaultDirCacheTTL)
	return &Service{
		deps:           deps,
		dirCache:       dirCache,
		watcherManager: newWatcherManager(dirCache, deps.Emitter),
	}
}

// resolveSessionWorkDir resolves the working directory for a session.
// For worktree sessions, returns the worktree path; otherwise returns root_path.
func (s *Service) resolveSessionWorkDir(sessionName string) (string, error) {
	return s.deps.ResolveSessionDir(sessionName, true)
}

// resolveSessionRepoDir resolves the git repository directory for a session.
// For worktree sessions, returns the repo_path (original repository).
// For regular sessions with root_path, returns root_path.
func (s *Service) resolveSessionRepoDir(sessionName string) (string, error) {
	return s.deps.ResolveSessionDir(sessionName, false)
}

// ResolveAndValidatePath resolves relPath relative to rootDir and validates
// that the result stays within rootDir boundaries.
func (s *Service) ResolveAndValidatePath(rootDir, relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	// filepath.IsLocal rejects "..", absolute paths, and OS-reserved names (e.g. NUL on Windows).
	if !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("path is not local (absolute, traversal, or reserved): %s", relPath)
	}

	// Resolve rootDir symlinks first for accurate containment check.
	resolvedRoot, rootErr := filepath.EvalSymlinks(rootDir)
	if rootErr != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", rootErr)
	}

	absPath := filepath.Join(resolvedRoot, cleaned)

	// Resolve symlinks and verify containment.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("path does not exist: %s", relPath)
		}
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	if !s.deps.IsPathWithinBase(resolved, resolvedRoot) {
		return "", fmt.Errorf("path escapes root directory: %s", relPath)
	}
	return resolved, nil
}

// ResolveAndValidateNewPath resolves relPath relative to rootDir and validates
// that the final target path would stay within rootDir boundaries, even when
// the target does not exist yet.
func (s *Service) ResolveAndValidateNewPath(rootDir, relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	if !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("path is not local (absolute, traversal, or reserved): %s", relPath)
	}

	resolvedRoot, rootErr := filepath.EvalSymlinks(rootDir)
	if rootErr != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", rootErr)
	}

	absPath := filepath.Join(resolvedRoot, cleaned)
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		if !s.deps.IsPathWithinBase(resolved, resolvedRoot) {
			return "", fmt.Errorf("path escapes root directory: %s", relPath)
		}
		return resolved, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	ancestor := absPath
	var missingParts []string
	for {
		info, err := os.Lstat(ancestor)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || info.IsDir() || ancestor == absPath {
				resolvedAncestor, resolveErr := filepath.EvalSymlinks(ancestor)
				if resolveErr != nil {
					return "", fmt.Errorf("failed to resolve path ancestor: %w", resolveErr)
				}

				resolvedTarget := resolvedAncestor
				for i := len(missingParts) - 1; i >= 0; i-- {
					resolvedTarget = filepath.Join(resolvedTarget, missingParts[i])
				}
				if !s.deps.IsPathWithinBase(resolvedTarget, resolvedRoot) {
					return "", fmt.Errorf("path escapes root directory: %s", relPath)
				}
				return resolvedTarget, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to inspect path ancestor: %w", err)
		}

		if ancestor == resolvedRoot {
			break
		}

		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			break
		}
		missingParts = append(missingParts, filepath.Base(ancestor))
		ancestor = parent
	}

	return "", fmt.Errorf("path escapes root directory: %s", relPath)
}

func normalizePanelPath(relPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(relPath))
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}

func parentPanelPath(relPath string) string {
	normalizedPath := normalizePanelPath(relPath)
	if normalizedPath == "" {
		return ""
	}

	parent := filepath.ToSlash(filepath.Dir(normalizedPath))
	if parent == "." {
		return ""
	}
	return parent
}

func (s *Service) invalidateDirCache(sessionName string, paths ...string) {
	if s.dirCache == nil {
		return
	}

	// Avoid repeated subtree scans when a mutation touches both a path and its
	// parent within the same operation; Invalidate itself is idempotent.
	invalidated := make(map[string]struct{}, len(paths)*2)
	for _, path := range paths {
		normalizedPath := normalizePanelPath(path)
		if _, ok := invalidated[normalizedPath]; !ok {
			s.dirCache.Invalidate(sessionName, normalizedPath)
			invalidated[normalizedPath] = struct{}{}
		}

		parentPath := parentPanelPath(normalizedPath)
		if _, ok := invalidated[parentPath]; ok {
			continue
		}
		s.dirCache.Invalidate(sessionName, parentPath)
		invalidated[parentPath] = struct{}{}
	}
}

func (s *Service) suppressWatcherPaths(sessionName string, paths ...string) {
	if s.watcherManager == nil {
		return
	}
	s.watcherManager.ignorePaths(sessionName, paths...)
}

func dirHasVisibleChildren(dirPath string) (bool, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	for {
		entries, readErr := dir.ReadDir(16)
		for _, entry := range entries {
			if entry.IsDir() && slices.Contains(excludedDirs, entry.Name()) {
				continue
			}
			return true, nil
		}
		if errors.Is(readErr, io.EOF) {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
	}
}

func validateMutationPath(relPath, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", fieldName)
	}

	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("%s must not refer to the session root", fieldName)
	}
	if !filepath.IsLocal(cleaned) {
		return "", fmt.Errorf("%s is not local (absolute, traversal, or reserved): %s", fieldName, relPath)
	}
	return cleaned, nil
}

func isRetryableFileError(err error) bool {
	if err == nil || runtime.GOOS != "windows" {
		return false
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch uint32(errno) {
		case uint32(errorAccessDenied), uint32(errorSharingViolation):
			return true
		}
	}
	return false
}

// retryFileOperation retries transient Windows file-lock failures with exponential
// backoff (10ms, 20ms, 40ms, 80ms, 160ms; about 310ms total wait).
func retryFileOperation(operation func() error, operationName string) error {
	var lastErr error
	for attempt := range maxRetries {
		lastErr = operation()
		if lastErr == nil {
			return nil
		}
		if !isRetryableFileError(lastErr) {
			return lastErr
		}
		if attempt < maxRetries-1 {
			time.Sleep(retryBaseDelay * time.Duration(1<<attempt))
		}
	}

	return fmt.Errorf("%s failed after %d retries: %w", operationName, maxRetries, lastErr)
}

// ListDir returns the contents of a directory within a session's working directory.
// Directories are listed first, sorted alphabetically. Lazy-loading friendly.
func (s *Service) ListDir(sessionName string, dirPath string) ([]FileEntry, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}

	normalizedDirPath := normalizePanelPath(dirPath)

	if s.dirCache != nil {
		if cachedEntries, ok := s.dirCache.Get(sessionName, normalizedDirPath); ok {
			return cachedEntries, nil
		}
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return nil, err
	}

	// For root listing, dirPath is empty or ".".
	targetDir := rootDir
	if normalizedDirPath != "" {
		resolved, resolveErr := s.ResolveAndValidatePath(rootDir, normalizedDirPath)
		if resolveErr != nil {
			return nil, resolveErr
		}
		targetDir = resolved
	}

	entries, readErr := os.ReadDir(targetDir)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read directory: %w", readErr)
	}

	var dirs []FileEntry
	var files []FileEntry
	count := 0
	for _, entry := range entries {
		if count >= maxDirEntries {
			slog.Warn("[DEVPANEL] directory entry limit reached",
				"dir", targetDir, "limit", maxDirEntries)
			break
		}

		name := entry.Name()

		// Skip excluded directories.
		if entry.IsDir() && slices.Contains(excludedDirs, name) {
			continue
		}

		// Dotfiles are intentionally visible in the developer panel.

		relPath, relErr := filepath.Rel(rootDir, filepath.Join(targetDir, name))
		if relErr != nil {
			slog.Debug("[DEVPANEL] failed to compute relative path", "name", name, "error", relErr)
			continue
		}
		// Normalize separators to forward slash for frontend consistency.
		relPath = filepath.ToSlash(relPath)

		fe := FileEntry{
			Name:  name,
			Path:  relPath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			hasChildren, hasChildrenErr := dirHasVisibleChildren(filepath.Join(targetDir, name))
			if hasChildrenErr != nil {
				slog.Debug("[DEVPANEL] failed to inspect directory children", "path", relPath, "error", hasChildrenErr)
				// Preserve the expander on transient filesystem errors so the
				// frontend stays on the safe side and can retry lazily.
				fe.HasChildren = true
			} else {
				fe.HasChildren = hasChildren
			}
		} else {
			if info, infoErr := entry.Info(); infoErr == nil {
				fe.Size = info.Size()
			}
		}

		if entry.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
		count++
	}

	// Sort: directories first (alphabetically), then files (alphabetically).
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	result := make([]FileEntry, 0, len(dirs)+len(files))
	result = append(result, dirs...)
	result = append(result, files...)

	if s.dirCache != nil {
		s.dirCache.Set(sessionName, normalizedDirPath, result)
	}

	return result, nil
}

// ReadFile reads a file within a session's working directory.
// Returns the file content with metadata. Files exceeding 1MB are truncated.
// Binary files are detected by scanning the first 8KB for NULL bytes.
func (s *Service) ReadFile(sessionName string, filePath string) (FileContent, error) {
	sessionName = strings.TrimSpace(sessionName)
	filePath = strings.TrimSpace(filePath)
	if sessionName == "" {
		return FileContent{}, errors.New("session name is required")
	}
	if filePath == "" {
		return FileContent{}, errors.New("file path is required")
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return FileContent{}, err
	}

	resolved, resolveErr := s.ResolveAndValidatePath(rootDir, filePath)
	if resolveErr != nil {
		return FileContent{}, resolveErr
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return FileContent{}, fmt.Errorf("failed to stat file: %w", statErr)
	}
	if info.IsDir() {
		return FileContent{}, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	result := FileContent{
		Path: filepath.ToSlash(filePath),
		Size: info.Size(),
	}

	f, openErr := os.Open(resolved)
	if openErr != nil {
		return FileContent{}, fmt.Errorf("failed to open file: %w", openErr)
	}
	defer func() { _ = f.Close() }()

	// Binary detection: read first probe bytes and scan for NULL bytes.
	probeSize := min(int64(binaryProbeSize), info.Size())
	probe := make([]byte, probeSize)
	probeN, probeReadErr := io.ReadFull(f, probe)
	if probeReadErr != nil && !errors.Is(probeReadErr, io.ErrUnexpectedEOF) {
		return FileContent{}, fmt.Errorf("failed to read file probe: %w", probeReadErr)
	}
	probe = probe[:probeN]

	if bytes.IndexByte(probe, 0) >= 0 {
		result.Binary = true
		result.Content = ""
		return result, nil
	}

	// Read remainder up to max file size using LimitReader.
	// Read one extra byte beyond the limit to detect truncation.
	remainLimit := max(maxFileSize-int64(probeN), 0)
	remainder, readErr := io.ReadAll(io.LimitReader(f, remainLimit+1))
	if readErr != nil {
		return FileContent{}, fmt.Errorf("failed to read file: %w", readErr)
	}

	data := append(probe, remainder...)

	// Truncate if total exceeds maxFileSize.
	if int64(len(data)) > maxFileSize {
		data = data[:maxFileSize]
		result.Truncated = true
	}

	result.Content = string(data)
	result.LineCount = strings.Count(result.Content, "\n") + 1
	return result, nil
}

// GetFileInfo returns metadata for a file-system entry within a session's working directory.
func (s *Service) GetFileInfo(sessionName, filePath string) (FileMetadata, error) {
	sessionName = strings.TrimSpace(sessionName)
	filePath = strings.TrimSpace(filePath)
	if sessionName == "" {
		return FileMetadata{}, errors.New("session name is required")
	}
	if filePath == "" {
		return FileMetadata{}, errors.New("file path is required")
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return FileMetadata{}, err
	}

	resolved, resolveErr := s.ResolveAndValidatePath(rootDir, filePath)
	if resolveErr != nil {
		return FileMetadata{}, resolveErr
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return FileMetadata{}, fmt.Errorf("failed to stat path: %w", statErr)
	}

	return FileMetadata{
		Path:  normalizePanelPath(filePath),
		Size:  info.Size(),
		IsDir: info.IsDir(),
	}, nil
}

// WriteFile writes content to a file within a session's working directory.
func (s *Service) WriteFile(sessionName, filePath, content string) (WriteFileResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return WriteFileResult{}, errors.New("session name is required")
	}

	cleanedPath, cleanErr := validateMutationPath(filePath, "file path")
	if cleanErr != nil {
		return WriteFileResult{}, cleanErr
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return WriteFileResult{}, err
	}

	resolved, resolveErr := s.ResolveAndValidateNewPath(rootDir, cleanedPath)
	if resolveErr != nil {
		return WriteFileResult{}, resolveErr
	}

	if info, statErr := os.Stat(resolved); statErr == nil && info.IsDir() {
		return WriteFileResult{}, fmt.Errorf("path is a directory, not a file: %s", cleanedPath)
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return WriteFileResult{}, fmt.Errorf("failed to stat path: %w", statErr)
	}

	parentDir := filepath.Dir(resolved)
	if mkdirErr := os.MkdirAll(parentDir, 0o755); mkdirErr != nil {
		return WriteFileResult{}, fmt.Errorf("failed to create parent directory: %w", mkdirErr)
	}

	s.suppressWatcherPaths(sessionName, cleanedPath)
	if writeErr := retryFileOperation(func() error {
		return os.WriteFile(resolved, []byte(content), 0o644)
	}, "write file"); writeErr != nil {
		return WriteFileResult{}, fmt.Errorf("failed to write file: %w", writeErr)
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return WriteFileResult{}, fmt.Errorf("failed to stat written file: %w", statErr)
	}

	s.invalidateDirCache(sessionName, cleanedPath)

	return WriteFileResult{
		Path: normalizePanelPath(cleanedPath),
		Size: info.Size(),
	}, nil
}

// CreateFile creates an empty file within a session's working directory.
func (s *Service) CreateFile(sessionName, filePath string) (WriteFileResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return WriteFileResult{}, errors.New("session name is required")
	}

	cleanedPath, cleanErr := validateMutationPath(filePath, "file path")
	if cleanErr != nil {
		return WriteFileResult{}, cleanErr
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return WriteFileResult{}, err
	}

	resolved, resolveErr := s.ResolveAndValidateNewPath(rootDir, cleanedPath)
	if resolveErr != nil {
		return WriteFileResult{}, resolveErr
	}

	if _, statErr := os.Stat(resolved); statErr == nil {
		return WriteFileResult{}, fmt.Errorf("path already exists: %s", cleanedPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return WriteFileResult{}, fmt.Errorf("failed to stat path: %w", statErr)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(resolved), 0o755); mkdirErr != nil {
		return WriteFileResult{}, fmt.Errorf("failed to create parent directory: %w", mkdirErr)
	}

	s.suppressWatcherPaths(sessionName, cleanedPath)
	if writeErr := retryFileOperation(func() error {
		return os.WriteFile(resolved, nil, 0o644)
	}, "create file"); writeErr != nil {
		return WriteFileResult{}, fmt.Errorf("failed to create file: %w", writeErr)
	}

	s.invalidateDirCache(sessionName, cleanedPath)

	return WriteFileResult{
		Path: normalizePanelPath(cleanedPath),
		Size: 0,
	}, nil
}

// CreateDirectory creates a directory within a session's working directory.
func (s *Service) CreateDirectory(sessionName, dirPath string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}

	cleanedPath, cleanErr := validateMutationPath(dirPath, "directory path")
	if cleanErr != nil {
		return cleanErr
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return err
	}

	resolved, resolveErr := s.ResolveAndValidateNewPath(rootDir, cleanedPath)
	if resolveErr != nil {
		return resolveErr
	}

	if _, statErr := os.Stat(resolved); statErr == nil {
		return fmt.Errorf("path already exists: %s", cleanedPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to stat path: %w", statErr)
	}

	s.suppressWatcherPaths(sessionName, cleanedPath)
	if mkdirErr := os.MkdirAll(resolved, 0o755); mkdirErr != nil {
		return fmt.Errorf("failed to create directory: %w", mkdirErr)
	}

	s.invalidateDirCache(sessionName, cleanedPath)

	return nil
}

// RenameFile renames or moves a file-system entry within a session's working directory.
func (s *Service) RenameFile(sessionName, oldPath, newPath string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}

	cleanedOldPath, cleanOldErr := validateMutationPath(oldPath, "old path")
	if cleanOldErr != nil {
		return cleanOldErr
	}
	cleanedNewPath, cleanNewErr := validateMutationPath(newPath, "new path")
	if cleanNewErr != nil {
		return cleanNewErr
	}
	if normalizePanelPath(cleanedOldPath) == normalizePanelPath(cleanedNewPath) {
		return errors.New("old path and new path must differ")
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return err
	}

	resolvedOldPath, resolveOldErr := s.ResolveAndValidatePath(rootDir, cleanedOldPath)
	if resolveOldErr != nil {
		return resolveOldErr
	}
	resolvedNewPath, resolveNewErr := s.ResolveAndValidateNewPath(rootDir, cleanedNewPath)
	if resolveNewErr != nil {
		return resolveNewErr
	}

	if _, statErr := os.Stat(resolvedOldPath); statErr != nil {
		return fmt.Errorf("failed to stat source path: %w", statErr)
	}
	if _, statErr := os.Stat(resolvedNewPath); statErr == nil {
		return fmt.Errorf("destination already exists: %s", cleanedNewPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to stat destination path: %w", statErr)
	}

	destinationDir := filepath.Dir(resolvedNewPath)
	info, statErr := os.Stat(destinationDir)
	if statErr != nil {
		return fmt.Errorf("destination directory does not exist: %w", statErr)
	}
	if !info.IsDir() {
		return fmt.Errorf("destination parent is not a directory: %s", filepath.ToSlash(filepath.Dir(cleanedNewPath)))
	}

	s.suppressWatcherPaths(sessionName, cleanedOldPath, cleanedNewPath)
	if renameErr := retryFileOperation(func() error {
		return os.Rename(resolvedOldPath, resolvedNewPath)
	}, "rename file"); renameErr != nil {
		return fmt.Errorf("failed to rename path: %w", renameErr)
	}

	s.invalidateDirCache(sessionName, cleanedOldPath, cleanedNewPath)

	return nil
}

// DeleteFile deletes a file-system entry within a session's working directory.
func (s *Service) DeleteFile(sessionName, filePath string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}

	cleanedPath, cleanErr := validateMutationPath(filePath, "file path")
	if cleanErr != nil {
		return cleanErr
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return err
	}

	resolved, resolveErr := s.ResolveAndValidatePath(rootDir, cleanedPath)
	if resolveErr != nil {
		return resolveErr
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return fmt.Errorf("failed to stat path: %w", statErr)
	}

	s.suppressWatcherPaths(sessionName, cleanedPath)
	var removeErr error
	if info.IsDir() {
		slog.Info("devpanel delete directory", "path", resolved)
		removeErr = retryFileOperation(func() error {
			return os.RemoveAll(resolved)
		}, "delete directory")
	} else {
		removeErr = retryFileOperation(func() error {
			return os.Remove(resolved)
		}, "delete file")
	}
	if removeErr != nil {
		return fmt.Errorf("failed to delete path: %w", removeErr)
	}

	s.invalidateDirCache(sessionName, cleanedPath)

	return nil
}

// GitLog returns the commit history for a session's repository.
// Results include parent hashes for graph rendering.
func (s *Service) GitLog(sessionName string, maxCount int, allBranches bool) ([]GitGraphCommit, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}
	if maxCount <= 0 {
		maxCount = 100
	}
	if maxCount > maxGitLogCount {
		maxCount = maxGitLogCount
	}

	repoDir, err := s.resolveSessionRepoDir(sessionName)
	if err != nil {
		return nil, err
	}

	if !gitpkg.IsGitRepository(repoDir) {
		return nil, fmt.Errorf("not a git repository: %s", repoDir)
	}

	// Use NUL-delimited format for safe parsing.
	// Fields: full_hash, short_hash, parents, subject, author_name, author_date_iso, decorations
	args := []string{
		"log",
		"--format=%H%x00%h%x00%P%x00%s%x00%an%x00%aI%x00%D",
		fmt.Sprintf("-n%d", maxCount),
	}
	if allBranches {
		args = append(args, "--all")
	}

	output, gitErr := gitpkg.RunGitCLIPublic(repoDir, args)
	if gitErr != nil {
		return nil, fmt.Errorf("git log failed: %w", gitErr)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]GitGraphCommit, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\x00", gitLogFieldCount)
		// decorations field (7th) is optional — empty when commit has no refs.
		if len(fields) < gitLogFieldCount-1 {
			slog.Debug("[DEVPANEL] skipping malformed git log line", "line", line)
			continue
		}

		commit := GitGraphCommit{
			FullHash:   fields[0],
			Hash:       fields[1],
			Subject:    fields[3],
			AuthorName: fields[4],
			AuthorDate: fields[5],
		}

		// Parse parent hashes (space-separated full hashes).
		if parentStr := strings.TrimSpace(fields[2]); parentStr != "" {
			commit.Parents = strings.Fields(parentStr)
		}

		// Parse refs/decorations.
		if len(fields) >= gitLogFieldCount && strings.TrimSpace(fields[6]) != "" {
			rawRefs := strings.Split(fields[6], ",")
			refs := make([]string, 0, len(rawRefs))
			for _, r := range rawRefs {
				r = strings.TrimSpace(r)
				// Strip "HEAD -> " prefix.
				r = strings.TrimPrefix(r, "HEAD -> ")
				if r != "" && r != "HEAD" {
					refs = append(refs, r)
				}
			}
			// Always include HEAD if present.
			if strings.Contains(fields[6], "HEAD") {
				refs = append([]string{"HEAD"}, refs...)
			}
			commit.Refs = refs
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// GitStatus returns the working tree status for a session's repository.
func (s *Service) GitStatus(sessionName string) (GitStatusResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return GitStatusResult{}, errors.New("session name is required")
	}

	workDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return GitStatusResult{}, err
	}

	if !gitpkg.IsGitRepository(workDir) {
		return GitStatusResult{}, fmt.Errorf("not a git repository")
	}

	result := GitStatusResult{
		Modified:   []string{},
		Staged:     []string{},
		Untracked:  []string{},
		Conflicted: []string{},
	}

	// Get branch name.
	// rev-parse --abbrev-ref returns "HEAD" for detached HEAD state.
	branchOutput, branchErr := gitpkg.RunGitCLIPublic(workDir, []string{"rev-parse", "--abbrev-ref", "HEAD"})
	if branchErr == nil {
		branch := strings.TrimSpace(string(branchOutput))
		if branch == "HEAD" {
			// Detached HEAD — use a sentinel value so the frontend can distinguish
			// this state from an error (where Branch remains "").
			result.Branch = DetachedHEADSentinel
		} else {
			result.Branch = branch
		}
	} else {
		slog.Warn("[DEVPANEL] failed to resolve branch name",
			"session", sessionName, "error", branchErr)
	}

	// Get status (porcelain format).
	// Use core.quotepath=false so non-ASCII paths (e.g. Japanese) are output
	// as real UTF-8 instead of octal-escaped sequences. This keeps paths
	// consistent with WorkingDiff which also uses quotepath=false.
	statusOutput, statusErr := gitpkg.RunGitCLIPublic(workDir, []string{
		"-c", "core.quotepath=false",
		"status", "--porcelain", "-b",
	})
	if statusErr != nil {
		return result, fmt.Errorf("git status failed: %w", statusErr)
	}

	for line := range strings.SplitSeq(string(statusOutput), "\n") {
		if len(line) < 4 {
			continue
		}
		// Skip branch header line.
		if strings.HasPrefix(line, "## ") {
			continue
		}
		indexStatus := line[0]
		workTreeStatus := line[1]
		// Git still quotes paths containing spaces, tabs, or backslashes.
		// decodeGitPathLiteral strips quotes and decodes escape sequences.
		rawPath := strings.TrimSpace(line[3:])
		filePath, ok := decodeGitPathLiteral(rawPath)
		if !ok {
			slog.Debug("[DEVPANEL] skipping malformed git status path", "raw", rawPath)
			continue
		}

		// Conflict detection: either side unmerged (UU, AU, UA, DU, UD),
		// both added (AA), or both deleted (DD).
		// Conflicted files are excluded from Staged/Modified to avoid double-counting.
		if indexStatus == 'U' || workTreeStatus == 'U' ||
			(indexStatus == 'A' && workTreeStatus == 'A') ||
			(indexStatus == 'D' && workTreeStatus == 'D') {
			result.Conflicted = append(result.Conflicted, filePath)
			continue
		}

		// Staged changes.
		// Safety net: exclude 'U' (conflict) in case the early-continue is refactored away.
		if indexStatus != ' ' && indexStatus != '?' && indexStatus != 'U' {
			result.Staged = append(result.Staged, filePath)
		}
		// Modified (unstaged) changes.
		if workTreeStatus == 'M' || workTreeStatus == 'D' {
			result.Modified = append(result.Modified, filePath)
		}
		// Untracked files.
		if indexStatus == '?' && workTreeStatus == '?' {
			result.Untracked = append(result.Untracked, filePath)
		}
	}

	// Get ahead/behind counts.
	revListOutput, revListErr := gitpkg.RunGitCLIPublic(workDir,
		[]string{"rev-list", "--left-right", "--count", "@{u}...HEAD"})
	if revListErr == nil {
		parts := strings.Fields(strings.TrimSpace(string(revListOutput)))
		if len(parts) == 2 {
			behindOK := true
			aheadOK := true
			if _, scanErr := fmt.Sscanf(parts[0], "%d", &result.Behind); scanErr != nil {
				slog.Warn("[DEVPANEL] failed to parse behind count", "raw", parts[0], "error", scanErr)
				behindOK = false
			}
			if _, scanErr := fmt.Sscanf(parts[1], "%d", &result.Ahead); scanErr != nil {
				slog.Warn("[DEVPANEL] failed to parse ahead count", "raw", parts[1], "error", scanErr)
				aheadOK = false
			}
			// Only mark upstream as configured when counts are reliably parsed.
			// A parse failure with UpstreamConfigured=true and Ahead/Behind=0
			// would mislead the frontend into showing "no diff" when the actual
			// counts are unknown.
			if behindOK && aheadOK {
				result.UpstreamConfigured = true
			}
		} else {
			slog.Debug("[DEVPANEL] unexpected rev-list output format", "session", sessionName, "raw", string(revListOutput))
		}
	} else {
		// Distinguish upstream-not-configured (normal) from other errors (unexpected).
		if gitpkg.IsNoUpstreamError(revListErr.Error()) {
			slog.Debug("[DEVPANEL] no upstream configured for ahead/behind counts",
				"session", sessionName)
		} else {
			slog.Warn("[DEVPANEL] unexpected error getting ahead/behind counts",
				"session", sessionName, "error", revListErr)
		}
	}

	return result, nil
}

// CommitDiff returns the unified diff for a specific commit.
func (s *Service) CommitDiff(sessionName string, commitHash string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	commitHash = strings.TrimSpace(commitHash)
	if sessionName == "" {
		return "", errors.New("session name is required")
	}
	if commitHash == "" {
		return "", errors.New("commit hash is required")
	}

	if err := gitpkg.ValidateCommitish(commitHash); err != nil {
		return "", fmt.Errorf("invalid commit hash: %w", err)
	}

	repoDir, err := s.resolveSessionRepoDir(sessionName)
	if err != nil {
		return "", err
	}

	if !gitpkg.IsGitRepository(repoDir) {
		return "", fmt.Errorf("not a git repository")
	}

	// SECURITY: commitHash is validated by ValidateCommitish above.
	// --root ensures root commits (no parent) also produce diff output.
	output, gitErr := gitpkg.RunGitCLIPublic(repoDir, []string{"diff-tree", "--root", "-p", commitHash, "--"})
	if gitErr != nil {
		return "", fmt.Errorf("git diff-tree failed: %w", gitErr)
	}

	diff := string(output)
	if len(diff) > maxDiffSize {
		diff = diff[:maxDiffSize] + "\n... (diff truncated)"
	}
	return diff, nil
}

// WorkingDiff returns the unified diff of working changes (staged + unstaged) vs HEAD,
// plus synthetic diffs for untracked (new) files.
func (s *Service) WorkingDiff(sessionName string) (WorkingDiffResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return WorkingDiffResult{}, errors.New("session name is required")
	}

	workDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return WorkingDiffResult{}, err
	}

	if !gitpkg.IsGitRepository(workDir) {
		// Not a git repository -- return empty result, no error.
		return WorkingDiffResult{}, nil
	}

	isFreshRepo, headProbeErr := detectFreshRepoState(workDir, gitpkg.RunGitCLIPublic)
	if headProbeErr != nil {
		return WorkingDiffResult{}, headProbeErr
	}

	var output []byte
	diffLoadIncomplete := false
	if isFreshRepo {
		slog.Debug("[DEVPANEL] repository has no commits yet; using staged-only diff path")
		var gitErr error
		output, gitErr = gitpkg.RunGitCLIPublic(workDir, []string{
			"-c", "core.quotepath=false",
			"diff", "--cached", "--no-color", "--",
		})
		if gitErr != nil {
			// Continue with untracked-only fallback while marking the response truncated.
			diffLoadIncomplete = true
			slog.Warn("[DEVPANEL] failed to load staged diff for fresh repo", "error", gitErr)
		}
	} else {
		var gitErr error
		output, gitErr = gitpkg.RunGitCLIPublic(workDir, []string{
			"-c", "core.quotepath=false",
			"diff", "HEAD", "--no-color", "--",
		})
		if gitErr != nil {
			return WorkingDiffResult{}, fmt.Errorf("git diff HEAD failed: %w", gitErr)
		}
	}

	raw := string(output)
	truncated := diffLoadIncomplete
	if len(raw) > maxDiffSize {
		raw = raw[:maxDiffSize]
		// Remove the last incomplete diff block to prevent partial entry
		// corruption. Find the last "diff --git " boundary and truncate there.
		if lastIdx := strings.LastIndex(raw, "diff --git "); lastIdx > 0 {
			raw = raw[:lastIdx]
		}
		truncated = true
	}

	files := parseWorkingDiff(raw)

	// Collect untracked files via git ls-files.
	untrackedFiles, hasLsErr := collectUntrackedFiles(workDir)
	if hasLsErr {
		truncated = true
	}
	// consumedSize tracks the assembled payload length after any main diff truncation.
	// This intentionally starts from len(raw), not len(output), because budget checks
	// must match what is actually returned to the frontend.
	consumedSize := len(raw)
	for _, relPath := range untrackedFiles {
		remainingBudget := maxDiffSize - consumedSize
		if remainingBudget <= 0 {
			truncated = true
			break
		}

		entries, budgetExceeded := s.buildUntrackedFileDiffsWithBudget(workDir, relPath, remainingBudget)
		for _, entry := range entries {
			consumedSize += len(entry.Diff)
			files = append(files, entry)
		}
		if budgetExceeded {
			truncated = true
			break
		}
	}

	totalAdded := 0
	totalDeleted := 0
	for _, f := range files {
		totalAdded += f.Additions
		totalDeleted += f.Deletions
	}

	return WorkingDiffResult{
		Files:        files,
		TotalAdded:   totalAdded,
		TotalDeleted: totalDeleted,
		Truncated:    truncated,
	}, nil
}

// ListBranches returns all branch names for a session's repository.
func (s *Service) ListBranches(sessionName string) ([]string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}

	repoDir, err := s.resolveSessionRepoDir(sessionName)
	if err != nil {
		return nil, err
	}

	if !gitpkg.IsGitRepository(repoDir) {
		return nil, fmt.Errorf("not a git repository")
	}

	output, gitErr := gitpkg.RunGitCLIPublic(repoDir,
		[]string{"branch", "--format=%(refname:short)", "-a"})
	if gitErr != nil {
		return nil, fmt.Errorf("git branch failed: %w", gitErr)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ---------------------------------------------------------------------------
// Git operations (stage, unstage, discard, commit, push, pull, fetch)
// ---------------------------------------------------------------------------

// validateGitFilePath validates a relative file path for git operations.
// Rejects empty, absolute, and path-traversal paths.
func validateGitFilePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("file path is required")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("file path must be relative: %s", path)
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsLocal(cleaned) {
		return fmt.Errorf("file path is not local: %s", path)
	}
	return nil
}

// resolveAndValidateGitSession validates the session name, resolves working
// directory, and verifies it is a git repository.
func (s *Service) resolveAndValidateGitSession(sessionName string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", errors.New("session name is required")
	}

	workDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return "", err
	}

	if !gitpkg.IsGitRepository(workDir) {
		return "", fmt.Errorf("not a git repository: %s", workDir)
	}

	return workDir, nil
}

// GitStage stages a file for commit (git add).
func (s *Service) GitStage(sessionName string, path string) error {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	if err := validateGitFilePath(path); err != nil {
		return err
	}

	// Use forward slash for git CLI consistency on Windows.
	gitPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))

	_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"add", "--", gitPath})
	if gitErr != nil {
		return fmt.Errorf("git add failed: %w", gitErr)
	}

	slog.Debug("[DEVPANEL-GIT] staged file", "session", sessionName, "path", gitPath)
	return nil
}

// GitUnstage unstages a file (git restore --staged).
// For fresh repositories (no commits), uses git rm --cached instead.
func (s *Service) GitUnstage(sessionName string, path string) error {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	if err := validateGitFilePath(path); err != nil {
		return err
	}

	gitPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))

	isFresh, probeErr := detectFreshRepoState(workDir, gitpkg.RunGitCLIPublic)
	if probeErr != nil {
		return fmt.Errorf("failed to detect repo state: %w", probeErr)
	}

	if isFresh {
		// Fresh repo has no HEAD to restore to; use git rm --cached.
		_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"rm", "--cached", "--", gitPath})
		if gitErr != nil {
			return fmt.Errorf("git rm --cached failed: %w", gitErr)
		}
	} else {
		_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"restore", "--staged", "--", gitPath})
		if gitErr != nil {
			return fmt.Errorf("git restore --staged failed: %w", gitErr)
		}
	}

	slog.Debug("[DEVPANEL-GIT] unstaged file", "session", sessionName, "path", gitPath)
	return nil
}

// GitDiscard discards working changes for a file.
// For tracked files, restores from index (git restore).
// For untracked files, removes the file (git clean -f).
func (s *Service) GitDiscard(sessionName string, path string) error {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	if err := validateGitFilePath(path); err != nil {
		return err
	}

	gitPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))

	// Check if file is untracked via git status --porcelain.
	statusOutput, statusErr := gitpkg.RunGitCLIPublic(workDir, []string{
		"status", "--porcelain", "--", gitPath,
	})
	if statusErr != nil {
		return fmt.Errorf("git status failed: %w", statusErr)
	}

	status := strings.TrimSpace(string(statusOutput))
	if strings.HasPrefix(status, "??") {
		// Untracked file — remove it.
		_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"clean", "-f", "--", gitPath})
		if gitErr != nil {
			return fmt.Errorf("git clean failed: %w", gitErr)
		}
	} else {
		// Tracked file — restore from index/HEAD.
		_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"restore", "--", gitPath})
		if gitErr != nil {
			return fmt.Errorf("git restore failed: %w", gitErr)
		}
	}

	slog.Debug("[DEVPANEL-GIT] discarded file", "session", sessionName, "path", gitPath)
	return nil
}

// GitStageAll stages all changes for commit (git add -A).
func (s *Service) GitStageAll(sessionName string) error {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"add", "-A"})
	if gitErr != nil {
		return fmt.Errorf("git add -A failed: %w", gitErr)
	}

	slog.Debug("[DEVPANEL-GIT] staged all changes", "session", sessionName)
	return nil
}

// GitUnstageAll unstages all staged changes.
// For fresh repositories, uses git rm -r --cached.
func (s *Service) GitUnstageAll(sessionName string) error {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	isFresh, probeErr := detectFreshRepoState(workDir, gitpkg.RunGitCLIPublic)
	if probeErr != nil {
		return fmt.Errorf("failed to detect repo state: %w", probeErr)
	}

	if isFresh {
		_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"rm", "-r", "--cached", "."})
		if gitErr != nil {
			return fmt.Errorf("git rm -r --cached failed: %w", gitErr)
		}
	} else {
		_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"reset", "HEAD"})
		if gitErr != nil {
			return fmt.Errorf("git reset HEAD failed: %w", gitErr)
		}
	}

	slog.Debug("[DEVPANEL-GIT] unstaged all changes", "session", sessionName)
	return nil
}

// GitCommit creates a commit with the currently staged changes.
// Returns the short hash and message of the created commit.
func (s *Service) GitCommit(sessionName string, message string) (CommitResult, error) {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return CommitResult{}, err
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return CommitResult{}, errors.New("commit message is required")
	}

	// Verify there are staged changes.
	_, diffErr := gitpkg.RunGitCLIPublic(workDir, []string{"diff", "--cached", "--quiet"})
	if diffErr == nil {
		// Exit code 0 means no staged changes.
		return CommitResult{}, errors.New("no staged changes to commit")
	}

	// Commit.
	_, commitErr := gitpkg.RunGitCLIPublic(workDir, []string{"commit", "-m", message})
	if commitErr != nil {
		return CommitResult{}, fmt.Errorf("git commit failed: %w", commitErr)
	}

	// Get the short hash of the created commit.
	hashOutput, hashErr := gitpkg.RunGitCLIPublic(workDir, []string{"rev-parse", "--short", "HEAD"})
	if hashErr != nil {
		slog.Warn("[DEVPANEL-GIT] commit succeeded but failed to read hash", "error", hashErr)
		return CommitResult{Message: firstLine(message)}, nil
	}

	hash := strings.TrimSpace(string(hashOutput))
	slog.Debug("[DEVPANEL-GIT] committed", "session", sessionName, "hash", hash)

	return CommitResult{
		Hash:    hash,
		Message: firstLine(message),
	}, nil
}

// GitPush pushes the current branch to its remote.
// If no upstream is set, automatically sets it with -u.
func (s *Service) GitPush(sessionName string) (PushResult, error) {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return PushResult{}, err
	}

	// Get current branch name.
	branchOutput, branchErr := gitpkg.RunGitCLIPublic(workDir, []string{"rev-parse", "--abbrev-ref", "HEAD"})
	if branchErr != nil {
		return PushResult{}, fmt.Errorf("failed to determine current branch: %w", branchErr)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Resolve configured remote for current branch, defaulting to "origin".
	remoteName, resolveErr := gitpkg.ResolveRemoteName(workDir, branch)
	if resolveErr != nil {
		return PushResult{}, fmt.Errorf("failed to resolve remote: %w", resolveErr)
	}

	// Pre-check upstream config to avoid locale-dependent error string matching.
	// git config returns exit code 1 when the key does not exist (expected).
	// Other errors (I/O, permissions, corrupt config) must abort the push to
	// prevent unintended tracking configuration changes.
	_, mergeErr := gitpkg.RunGitCLIPublic(workDir, []string{
		"config", fmt.Sprintf("branch.%s.merge", branch),
	})
	hasUpstream := mergeErr == nil
	if mergeErr != nil && !gitpkg.IsGitConfigKeyNotFound(mergeErr) {
		return PushResult{}, fmt.Errorf("failed to check upstream config for branch %q: %w", branch, mergeErr)
	}

	if hasUpstream {
		// Normal push — upstream is configured.
		_, pushErr := gitpkg.RunGitCLIPublic(workDir, []string{"push", remoteName, "HEAD"})
		if pushErr != nil {
			return PushResult{}, fmt.Errorf("git push failed: %w", pushErr)
		}
		slog.Debug("[DEVPANEL-GIT] pushed", "session", sessionName, "branch", branch)
		return PushResult{
			RemoteName: remoteName,
			BranchName: branch,
		}, nil
	}

	// No upstream — push with -u directly (no retry needed).
	slog.Debug("[DEVPANEL-GIT] no upstream configured, pushing with -u", "branch", branch)
	_, pushErr := gitpkg.RunGitCLIPublic(workDir, []string{"push", "-u", remoteName, "HEAD"})
	if pushErr != nil {
		return PushResult{}, fmt.Errorf("git push failed: %w", pushErr)
	}

	slog.Debug("[DEVPANEL-GIT] pushed with upstream set", "session", sessionName, "branch", branch)
	return PushResult{
		RemoteName:  remoteName,
		BranchName:  branch,
		UpstreamSet: true,
	}, nil
}

// GitPull pulls changes from the remote for the current branch.
// Uses --ff-only to prevent unintended merge commits; non-fast-forward
// situations require manual resolution in the terminal.
func (s *Service) GitPull(sessionName string) (PullResult, error) {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return PullResult{}, err
	}

	output, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"pull", "--ff-only"})
	if gitErr != nil {
		if isNonFastForwardError(workDir, gitErr) {
			return PullResult{}, fmt.Errorf(
				"pull failed: fast-forward is not possible. Please merge or rebase manually in the terminal")
		}
		return PullResult{}, fmt.Errorf("git pull failed: %w", gitErr)
	}

	summary := strings.TrimSpace(string(output))
	updated := !strings.Contains(summary, "Already up to date")

	slog.Debug("[DEVPANEL-GIT] pulled", "session", sessionName, "updated", updated)
	return PullResult{
		Updated: updated,
		Summary: summary,
	}, nil
}

// GitFetch fetches from all remotes and prunes deleted references.
func (s *Service) GitFetch(sessionName string) error {
	workDir, err := s.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	_, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"fetch", "--prune"})
	if gitErr != nil {
		return fmt.Errorf("git fetch failed: %w", gitErr)
	}

	slog.Debug("[DEVPANEL-GIT] fetched", "session", sessionName)
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// detectFreshRepoState checks whether a repository has any commits.
func detectFreshRepoState(workDir string, runGit func(string, []string) ([]byte, error)) (bool, error) {
	if runGit == nil {
		return false, errors.New("git runner is required")
	}

	_, headErr := runGit(workDir, []string{"rev-parse", "--verify", "--quiet", "HEAD"})
	if headErr == nil {
		return false, nil
	}

	statusOutput, statusErr := runGit(workDir, []string{"status", "--porcelain", "--branch"})
	if statusErr != nil {
		return false, fmt.Errorf("failed to verify HEAD state: %w", errors.Join(headErr, statusErr))
	}
	if isFreshRepoStatusOutput(statusOutput) {
		return true, nil
	}

	// HEAD verification can fail for reasons other than "no commits yet"
	// (for example repository corruption or execution issues). Do not silently
	// downgrade those cases to fresh-repo fallback.
	return false, fmt.Errorf("failed to verify HEAD commit: %w", headErr)
}

// isNonFastForwardError detects whether a git pull --ff-only error indicates
// a non-fast-forward divergence.
//
// Primary: English error message matching (locale-dependent).
// Fallback: locale-independent ancestry check via git merge-base.
func isNonFastForwardError(workDir string, gitErr error) bool {
	// Primary: match known English error messages.
	errMsg := strings.ToLower(gitErr.Error())
	if strings.Contains(errMsg, "not possible to fast-forward") ||
		strings.Contains(errMsg, "cannot fast-forward") {
		return true
	}

	// Locale-independent fallback: check if HEAD can be fast-forwarded to @{u}.
	// merge-base --is-ancestor exits with code 1 specifically when HEAD is NOT
	// an ancestor of @{u}, confirming divergence.
	// Other exit codes (128 = no upstream, corrupt repo, etc.) are not treated
	// as non-fast-forward to avoid masking unrelated errors.
	_, probeErr := gitpkg.RunGitCLIPublic(workDir, []string{
		"merge-base", "--is-ancestor", "HEAD", "@{u}",
	})
	if probeErr != nil {
		var exitErr *exec.ExitError
		if errors.As(probeErr, &exitErr) && exitErr.ExitCode() == 1 {
			return true
		}
	}

	return false
}

func isFreshRepoStatusOutput(statusOutput []byte) bool {
	if len(statusOutput) == 0 {
		return false
	}

	normalized := strings.ReplaceAll(string(statusOutput), "\r\n", "\n")
	for line := range strings.SplitSeq(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "## No commits yet on ") ||
			strings.HasPrefix(line, "## Initial commit on ")
	}
	return false
}

// collectUntrackedFiles returns the list of untracked file paths from git ls-files.
func collectUntrackedFiles(workDir string) ([]string, bool) {
	// git ls-files --others --exclude-standard is preferred over git status -uall:
	// - Always returns individual file paths (not directory entries)
	// - Behavior is stable across git versions
	// - Correctly applies .gitignore rules
	output, err := gitpkg.RunGitCLIPublic(workDir, []string{
		"-c", "core.quotepath=false",
		"ls-files", "--others", "--exclude-standard", "-z",
	})
	if err != nil {
		slog.Warn("[DEVPANEL] git ls-files for untracked files failed", "error", err)
		return nil, true
	}

	return parseNULSeparatedGitPaths(output), false
}

func parseNULSeparatedGitPaths(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}

	entries := strings.Split(string(raw), "\x00")
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if len(paths) >= maxUntrackedFilePaths {
			slog.Warn("[DEVPANEL] untracked path limit reached",
				"limit", maxUntrackedFilePaths,
				"dropped", len(entries)-len(paths))
			break
		}
		paths = append(paths, filepath.ToSlash(entry))
	}
	return paths
}

// buildUntrackedFileDiffs creates synthetic diff entries for an untracked path.
// If the path is a directory, it recurses into it and produces entries for each file.
// Returns nil for binary/too-large/unreadable files.
func (s *Service) buildUntrackedFileDiffs(workDir, relPath string) []WorkingDiffFile {
	results, _ := s.buildUntrackedFileDiffsWithBudget(workDir, relPath, -1)
	return results
}

func (s *Service) buildUntrackedFileDiffsWithBudget(workDir, relPath string, remainingBudget int) ([]WorkingDiffFile, bool) {
	resolvedBase, baseErr := filepath.EvalSymlinks(workDir)
	if baseErr != nil {
		slog.Warn("[DEVPANEL] failed to resolve workDir while parsing untracked files", "error", baseErr)
		return nil, false
	}

	absPath := filepath.Join(workDir, filepath.FromSlash(relPath))

	info, err := os.Lstat(absPath)
	if err != nil {
		slog.Debug("[DEVPANEL] failed to stat untracked path", "path", relPath, "error", err)
		return nil, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Debug("[DEVPANEL] skipping symlink in untracked files", "path", relPath)
		return nil, false
	}

	if !info.IsDir() {
		// Pass Lstat metadata to avoid redundant stat calls per file.
		entry := s.buildUntrackedFileDiffSingleWithResolvedBase(workDir, relPath, resolvedBase, info)
		if entry == nil {
			return nil, false
		}
		if remainingBudget >= 0 && len(entry.Diff) > remainingBudget {
			return nil, true
		}
		return []WorkingDiffFile{*entry}, false
	}

	// Directory: walk recursively and collect individual files.
	var results []WorkingDiffFile
	budgetExceeded := false
	walkErr := filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("[DEVPANEL] failed to walk untracked path", "path", path, "error", err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// Skip excluded directories inside the walk.
			if slices.Contains(excludedDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if remainingBudget == 0 {
			budgetExceeded = true
			return fs.SkipAll
		}
		rel, relErr := filepath.Rel(workDir, path)
		if relErr != nil {
			return nil
		}
		normalized := filepath.ToSlash(rel)

		info, infoErr := d.Info()
		if infoErr != nil {
			slog.Debug("[DEVPANEL] failed to read file info while walking untracked files",
				"path", normalized, "error", infoErr)
			return nil
		}
		entry := s.buildUntrackedFileDiffSingleWithResolvedBase(workDir, normalized, resolvedBase, info)
		if entry != nil {
			if remainingBudget >= 0 && len(entry.Diff) > remainingBudget {
				budgetExceeded = true
				return fs.SkipAll
			}
			results = append(results, *entry)
			if remainingBudget >= 0 {
				remainingBudget -= len(entry.Diff)
			}
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
		slog.Warn("[DEVPANEL] WalkDir completed with error", "path", absPath, "error", walkErr)
	}
	return results, budgetExceeded
}

// buildUntrackedFileDiffSingle creates a synthetic diff entry for a single untracked file.
// Returns nil if the file is binary, too large, or cannot be read.
// Accepts an optional pre-fetched os.FileInfo (expected to come from os.Lstat)
// to avoid redundant filesystem metadata lookups.
func (s *Service) buildUntrackedFileDiffSingle(workDir, relPath string, cachedInfo ...os.FileInfo) *WorkingDiffFile {
	resolvedBase, baseErr := filepath.EvalSymlinks(workDir)
	if baseErr != nil {
		slog.Warn("[DEVPANEL] failed to resolve workDir while parsing untracked file", "error", baseErr)
		return nil
	}
	return s.buildUntrackedFileDiffSingleWithResolvedBase(workDir, relPath, resolvedBase, cachedInfo...)
}

func (s *Service) buildUntrackedFileDiffSingleWithResolvedBase(workDir, relPath, resolvedBase string, cachedInfo ...os.FileInfo) *WorkingDiffFile {
	absPath := filepath.Join(workDir, filepath.FromSlash(relPath))

	// Lexical guard before any file access.
	if !s.deps.IsPathWithinBase(absPath, workDir) {
		slog.Warn("[DEVPANEL] untracked path escapes workDir", "path", relPath)
		return nil
	}

	// Optional fast-path metadata from caller. At most one value is used.
	// Expected contract: cachedInfo[0] is an os.Lstat result for absPath.
	var info os.FileInfo
	if len(cachedInfo) > 0 && cachedInfo[0] != nil {
		info = cachedInfo[0]
	} else {
		lstatInfo, err := os.Lstat(absPath)
		if err != nil {
			slog.Debug("[DEVPANEL] failed to lstat untracked file", "path", relPath, "error", err)
			return nil
		}
		info = lstatInfo
	}
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Debug("[DEVPANEL] skipping symlink in untracked files", "path", relPath)
		return nil
	}
	if info.IsDir() {
		return nil
	}

	// Filesystem guard: resolve both base and target path and ensure containment
	// after symlink resolution (covers path traversal via symlink hops).
	resolvedAbs, evalErr := filepath.EvalSymlinks(absPath)
	if evalErr != nil {
		slog.Debug("[DEVPANEL] failed to resolve untracked path", "path", relPath, "error", evalErr)
		return nil
	}
	if !s.deps.IsPathWithinBase(resolvedAbs, resolvedBase) {
		slog.Warn("[DEVPANEL] untracked path escapes workDir after symlink resolution", "path", relPath)
		return nil
	}

	// Skip files that are too large.
	if info.Size() > maxUntrackedFileSize {
		slog.Debug("[DEVPANEL] skipping large untracked file", "path", relPath, "size", info.Size())
		return nil
	}

	f, err := os.Open(resolvedAbs)
	if err != nil {
		slog.Debug("[DEVPANEL] failed to open untracked file", "path", relPath, "error", err)
		return nil
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Debug("[DEVPANEL] failed to close untracked file", "path", relPath, "error", closeErr)
		}
	}()

	// Defensive hard cap: protects against TOCTOU where file size grows between
	// metadata check and read time.
	data, err := io.ReadAll(io.LimitReader(f, maxUntrackedFileSize+1))
	if err != nil {
		slog.Debug("[DEVPANEL] failed to read untracked file", "path", relPath, "error", err)
		return nil
	}
	if int64(len(data)) > maxUntrackedFileSize {
		slog.Debug("[DEVPANEL] skipping untracked file that exceeded size limit during read",
			"path", relPath, "limit", maxUntrackedFileSize)
		return nil
	}

	// Binary detection: scan for NULL bytes in first probe bytes.
	probeSize := min(len(data), binaryProbeSize)
	if bytes.IndexByte(data[:probeSize], 0) >= 0 {
		return nil
	}

	content := string(data)
	// Normalize Windows line endings for consistent line splitting.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Build synthetic unified diff.
	var sb strings.Builder
	normalizedPath := filepath.ToSlash(relPath)

	// Quote paths containing spaces or control characters to produce
	// Git-compatible diff headers. Git uses C-style quoting for such paths.
	diffPath := normalizedPath
	if needsGitQuoting(diffPath) {
		diffPath = strconv.Quote(diffPath)
	}

	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", diffPath, diffPath)
	sb.WriteString("new file mode 100644\n")
	sb.WriteString("index 0000000..0000000\n")
	sb.WriteString("--- /dev/null\n")
	fmt.Fprintf(&sb, "+++ b/%s\n", diffPath)
	// Empty files (len(lines) == 0) produce no hunk header or body,
	// matching Git's behavior for new empty files.
	if len(lines) == 0 {
		return &WorkingDiffFile{
			Path:      normalizedPath,
			OldPath:   "",
			Status:    WorkingDiffStatusUntracked,
			Additions: 0,
			Deletions: 0,
			Diff:      sb.String(),
		}
	}
	fmt.Fprintf(&sb, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		sb.WriteByte('+')
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return &WorkingDiffFile{
		Path:      normalizedPath,
		OldPath:   "",
		Status:    WorkingDiffStatusUntracked,
		Additions: len(lines),
		Deletions: 0,
		Diff:      sb.String(),
	}
}

// parseWorkingDiff parses a unified diff output into per-file WorkingDiffFile entries.
func parseWorkingDiff(raw string) []WorkingDiffFile {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	// NOTE: This parser handles standard unified diff blocks ("diff --git ...").
	// Combined diff formats (e.g. merge combined hunks) are out of scope.

	// Normalize Windows line endings for robust path extraction.
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	blocks := splitUnifiedDiffBlocks(raw)
	if len(blocks) == 0 {
		return nil
	}

	var files []WorkingDiffFile

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}
		header := strings.TrimPrefix(lines[0], "diff --git ")
		if header == lines[0] {
			continue
		}

		oldPath, newPath, ok := parseDiffHeaderPaths(header)
		if !ok {
			slog.Debug("[DEVPANEL] skipping diff block with malformed header", "header", header)
			continue
		}

		file := WorkingDiffFile{
			Path:    filepath.ToSlash(newPath),
			OldPath: filepath.ToSlash(oldPath),
			Diff:    block,
		}

		// Detect status and count additions/deletions.
		file.Status = WorkingDiffStatusModified // default
		for _, line := range lines {
			if strings.HasPrefix(line, "new file mode") {
				file.Status = WorkingDiffStatusAdded
			} else if strings.HasPrefix(line, "deleted file mode") {
				file.Status = WorkingDiffStatusDeleted
			} else if after, ok := strings.CutPrefix(line, "rename from "); ok {
				if renamedFrom, decoded := decodeGitPathLiteral(after); decoded {
					file.OldPath = filepath.ToSlash(renamedFrom)
				}
				file.Status = WorkingDiffStatusRenamed
			} else if after, ok := strings.CutPrefix(line, "rename to "); ok {
				if renamedTo, decoded := decodeGitPathLiteral(after); decoded {
					file.Path = filepath.ToSlash(renamedTo)
				}
				file.Status = WorkingDiffStatusRenamed
			} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				file.Additions++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				file.Deletions++
			}
		}

		files = append(files, file)
	}

	return files
}

func splitUnifiedDiffBlocks(raw string) []string {
	const marker = "diff --git "
	var blocks []string

	start := -1
	searchFrom := 0
	for {
		idx := strings.Index(raw[searchFrom:], marker)
		if idx < 0 {
			break
		}
		idx += searchFrom
		// Treat "diff --git " as a block header only when it appears at line start.
		if idx > 0 && raw[idx-1] != '\n' {
			searchFrom = idx + len(marker)
			continue
		}
		if start >= 0 {
			blocks = append(blocks, strings.TrimRight(raw[start:idx], "\n"))
		}
		start = idx
		searchFrom = idx + len(marker)
	}

	if start >= 0 {
		blocks = append(blocks, strings.TrimRight(raw[start:], "\n"))
	}

	return blocks
}

// parseDiffHeaderPaths extracts old/new paths from a "diff --git" header line.
// Assumes default git diff format with "a/" and "b/" prefixes (NOT --no-prefix).
//
// For renames (old != new), the symmetry heuristic cannot match because
// the two path segments differ. The fallback splits at the first " b/", which may
// produce incorrect results when the old path itself contains " b/" (e.g.,
// "a/lib b/foo b/lib c/bar"). Callers must apply rename from/rename to lines
// (parsed in parseWorkingDiff) to override these potentially inaccurate paths.
func parseDiffHeaderPaths(header string) (oldPath, newPath string, ok bool) {
	header = strings.TrimSpace(header)

	// Quoted paths (handles special chars, spaces, non-ASCII).
	if strings.HasPrefix(header, "\"") {
		return parseDiffHeaderPathsQuoted(header)
	}

	// Unquoted paths: header format is "a/<old> b/<new>".
	if !strings.HasPrefix(header, "a/") {
		return "", "", false
	}

	// For non-rename diffs (majority), old == new, so header = "a/<P> b/<P>".
	// Use path-length symmetry to find the split point.
	// Even total lengths (after removing "a/") intentionally fail this check and
	// fall back to separator-based parsing below.
	content := header[2:] // strip "a/"
	if len(content) >= 3 {
		pathLen := (len(content) - 3) / 2
		if 2*pathLen+3 == len(content) {
			candidate := content[:pathLen]
			if content[pathLen:pathLen+3] == " b/" && content[pathLen+3:] == candidate {
				return candidate, candidate, true
			}
		}
	}

	// Fallback for renames or asymmetric paths: split at first " b/".
	// For renames, the result here is a best-effort guess that will be
	// overridden by "rename from"/"rename to" lines in parseWorkingDiff.
	sepIdx := strings.Index(header, " b/")
	if sepIdx < 0 {
		return "", "", false
	}
	return header[2:sepIdx], header[sepIdx+3:], true
}

// parseDiffHeaderPathsQuoted parses diff header paths where at least one path is quoted.
func parseDiffHeaderPathsQuoted(header string) (oldPath string, newPath string, ok bool) {
	oldSpec, rest, specOk := takeDiffHeaderPathSpec(header)
	if !specOk {
		return "", "", false
	}

	newSpec, _, specOk := takeDiffHeaderPathSpec(rest)
	if !specOk {
		return "", "", false
	}

	oldLiteral, decOk := decodeGitPathLiteral(oldSpec)
	if !decOk || !strings.HasPrefix(oldLiteral, "a/") {
		return "", "", false
	}

	newLiteral, decOk := decodeGitPathLiteral(newSpec)
	if !decOk || !strings.HasPrefix(newLiteral, "b/") {
		return "", "", false
	}

	return strings.TrimPrefix(oldLiteral, "a/"), strings.TrimPrefix(newLiteral, "b/"), true
}

func takeDiffHeaderPathSpec(input string) (pathSpec string, rest string, ok bool) {
	trimmed := strings.TrimLeft(input, " \t")
	if trimmed == "" {
		return "", "", false
	}

	if trimmed[0] != '"' {
		end := strings.IndexAny(trimmed, " \t")
		if end == -1 {
			return trimmed, "", true
		}
		return trimmed[:end], trimmed[end:], true
	}

	escaped := false
	for i := 1; i < len(trimmed); i++ {
		ch := trimmed[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return trimmed[:i+1], trimmed[i+1:], true
		}
	}

	return "", "", false
}

func decodeGitPathLiteral(pathSpec string) (string, bool) {
	pathSpec = strings.TrimSpace(pathSpec)
	if pathSpec == "" {
		return "", false
	}

	if !strings.HasPrefix(pathSpec, "\"") {
		return pathSpec, true
	}

	// Git's quoted path format uses C-style escapes, which are compatible with
	// strconv.Unquote for the escape sequences Git emits in diff headers.
	decoded, err := strconv.Unquote(pathSpec)
	if err != nil {
		return "", false
	}
	return decoded, true
}

// needsGitQuoting returns true when a path contains characters that require
// C-style quoting in Git diff headers (spaces, control characters, backslashes,
// double-quotes, or non-ASCII bytes).
func needsGitQuoting(path string) bool {
	for _, b := range []byte(path) {
		if b == ' ' || b == '"' || b == '\\' || b < 0x20 || b >= 0x80 {
			return true
		}
	}
	return false
}

// firstLine returns the first line of a multiline string.
func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

// ── File Search ──

// maxSearchResults is the maximum number of file results returned by SearchFiles.
const maxSearchResults = 200

// maxContentLinesPerFile is the maximum number of matching lines per file in content search.
const maxContentLinesPerFile = 3

// maxContentLineLength is the maximum length of a single content match line.
const maxContentLineLength = 500

// maxSearchFileSize is the maximum file size for manual content search (512 KB).
const maxSearchFileSize int64 = 512 * 1024

// SearchFiles searches for files by name and content within a session's working directory.
// Returns matching files with optional content match lines. Empty query returns empty results.
func (s *Service) SearchFiles(sessionName, query string) ([]SearchFileResult, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchFileResult{}, nil
	}

	rootDir, err := s.resolveSessionWorkDir(sessionName)
	if err != nil {
		return nil, err
	}

	var results []SearchFileResult

	if gitpkg.IsGitRepository(rootDir) {
		// Git repo: WalkDir for name matches + git grep for content matches.
		nameMatches, walkErr := s.searchFilesByName(rootDir, query)
		if walkErr != nil {
			slog.Warn("[DEVPANEL] file name search failed", "error", walkErr)
			nameMatches = nil
		}
		contentMatches := s.searchContentWithGitGrep(rootDir, query)
		results = mergeSearchResults(nameMatches, contentMatches)
	} else {
		// Non-git: single WalkDir handles both name and content matching.
		var walkErr error
		results, walkErr = s.searchFilesManual(rootDir, query)
		if walkErr != nil {
			slog.Warn("[DEVPANEL] manual file search failed", "error", walkErr)
		}
	}

	// Sort: name matches first, then alphabetical by path.
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsNameMatch != results[j].IsNameMatch {
			return results[i].IsNameMatch
		}
		return results[i].Path < results[j].Path
	})

	// Truncate to maxSearchResults.
	if len(results) > maxSearchResults {
		results = results[:maxSearchResults]
	}

	return results, nil
}

// searchFilesByName walks the directory tree and finds files whose name contains the query.
func (s *Service) searchFilesByName(rootDir, query string) ([]SearchFileResult, error) {
	lowerQuery := strings.ToLower(query)
	var results []SearchFileResult

	walkErr := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		if d.IsDir() {
			if slices.Contains(excludedDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if len(results) >= maxSearchResults {
			return filepath.SkipAll
		}

		name := d.Name()
		if !strings.Contains(strings.ToLower(name), lowerQuery) {
			return nil
		}

		relPath, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return nil
		}

		results = append(results, SearchFileResult{
			Path:         filepath.ToSlash(relPath),
			Name:         name,
			IsNameMatch:  true,
			ContentLines: []SearchContentLine{},
		})
		return nil
	})

	return results, walkErr
}

// searchContentWithGitGrep runs `git grep` to find content matches in tracked files.
func (s *Service) searchContentWithGitGrep(rootDir, query string) []SearchFileResult {
	args := []string{
		"grep", "-n", "-i", "-F",
		"--max-count=" + strconv.Itoa(maxContentLinesPerFile),
		"-I", // skip binary files
		"--", query,
	}

	out, err := gitpkg.RunGitCLIPublic(rootDir, args)
	if err != nil {
		// Exit code 1 = no match, which is normal.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		slog.Warn("[DEVPANEL] git grep failed", "error", err)
		return nil
	}

	return parseGitGrepOutput(out)
}

// parseGitGrepOutput parses `git grep -n` output (path:line:content format)
// into SearchFileResult entries grouped by file path.
func parseGitGrepOutput(out []byte) []SearchFileResult {
	resultMap := make(map[string]*SearchFileResult)
	var order []string

	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n\r"), "\n") {
		if line == "" {
			continue
		}

		// Format: path:lineNum:content
		// Two-step Cut to handle colons within content.
		filePath, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		lineNumStr, content, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}

		lineNum, parseErr := strconv.Atoi(lineNumStr)
		if parseErr != nil {
			continue
		}

		// Truncate content line at rune boundary.
		content = truncateUTF8(content, maxContentLineLength)

		existing, exists := resultMap[filePath]
		if !exists {
			result := &SearchFileResult{
				Path:         filepath.ToSlash(filePath),
				Name:         filepath.Base(filePath),
				ContentLines: []SearchContentLine{},
			}
			resultMap[filePath] = result
			order = append(order, filePath)
			existing = result
		}

		if len(existing.ContentLines) < maxContentLinesPerFile {
			existing.ContentLines = append(existing.ContentLines, SearchContentLine{
				Line:    lineNum,
				Content: content,
			})
		}
	}

	results := make([]SearchFileResult, 0, len(order))
	for _, p := range order {
		if len(results) >= maxSearchResults {
			break
		}
		results = append(results, *resultMap[p])
	}
	return results
}

// searchFilesManual searches for files by name and content in a single directory walk.
// Used for non-git directories to avoid walking the tree twice.
func (s *Service) searchFilesManual(rootDir, query string) ([]SearchFileResult, error) {
	lowerQuery := strings.ToLower(query)
	var results []SearchFileResult

	walkErr := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		if d.IsDir() {
			if slices.Contains(excludedDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if len(results) >= maxSearchResults {
			return filepath.SkipAll
		}

		name := d.Name()
		isNameMatch := strings.Contains(strings.ToLower(name), lowerQuery)

		// Content search: read file and find matching lines.
		var contentLines []SearchContentLine
		info, infoErr := d.Info()
		if infoErr == nil && info.Size() <= maxSearchFileSize {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				probeLen := min(len(data), binaryProbeSize)
				if bytes.IndexByte(data[:probeLen], 0) < 0 {
					lineNum := 0
					for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
						lineNum++
						if len(contentLines) >= maxContentLinesPerFile {
							break
						}
						if strings.Contains(strings.ToLower(line), lowerQuery) {
							contentLines = append(contentLines, SearchContentLine{
								Line:    lineNum,
								Content: truncateUTF8(line, maxContentLineLength),
							})
						}
					}
				}
			}
		}

		if !isNameMatch && len(contentLines) == 0 {
			return nil
		}

		relPath, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return nil
		}

		if contentLines == nil {
			contentLines = []SearchContentLine{}
		}

		results = append(results, SearchFileResult{
			Path:         filepath.ToSlash(relPath),
			Name:         name,
			IsNameMatch:  isNameMatch,
			ContentLines: contentLines,
		})

		return nil
	})

	return results, walkErr
}

// mergeSearchResults combines name-match and content-match results,
// deduplicating by path. When a file appears in both, the entry is merged.
func mergeSearchResults(nameMatches, contentMatches []SearchFileResult) []SearchFileResult {
	indexed := make(map[string]*SearchFileResult, len(nameMatches)+len(contentMatches))
	var order []string

	for i := range nameMatches {
		m := &nameMatches[i]
		indexed[m.Path] = m
		order = append(order, m.Path)
	}

	for i := range contentMatches {
		c := &contentMatches[i]
		if existing, ok := indexed[c.Path]; ok {
			// Merge: keep name match flag and add content lines.
			existing.ContentLines = c.ContentLines
		} else {
			indexed[c.Path] = c
			order = append(order, c.Path)
		}
	}

	results := make([]SearchFileResult, 0, len(order))
	for _, p := range order {
		results = append(results, *indexed[p])
	}
	return results
}

// truncateUTF8 truncates s to at most maxBytes, backing up to a valid rune
// boundary so that multi-byte characters are never split.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}
