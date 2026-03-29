package worktree

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ===========================================================================
// File/directory copy functions
// ===========================================================================

func (s *Service) copyConfigEntriesToWorktree(
	repoPath, wtPath string,
	entries []string,
	entryKind string,
	copyFn func(repoBase, wtBase, entry string) bool,
) []string {
	var failures []string
	if len(entries) == 0 {
		return failures
	}
	repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoPath)
	if repoErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve repository base path for copy",
			"repoPath", repoPath, "entryKind", entryKind, "error", repoErr)
		return normalizeCopyFailures(entries)
	}
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtPath)
	if wtErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve worktree base path for copy",
			"worktreePath", wtPath, "entryKind", entryKind, "error", wtErr)
		return normalizeCopyFailures(entries)
	}
	for _, entry := range entries {
		if failed := copyFn(repoBase, wtBase, entry); failed {
			failures = append(failures, entry)
		}
	}
	return failures
}

// CopyConfigFilesToWorktree copies configured files (e.g. .env) from the
// repository root to the worktree. Returns a list of files that failed to copy.
// Missing source files are silently skipped (common for optional files like .env).
func (s *Service) CopyConfigFilesToWorktree(repoPath, wtPath string, files []string) []string {
	return s.copyConfigEntriesToWorktree(repoPath, wtPath, files, "file", s.copyConfigFileToWorktree)
}

func validateAndResolveSourceEntry(
	repoBase, wtBase, entry, configKey, fieldKey string,
) (resolvedSrc, dstPath string, canProcess, failed bool) {
	cleaned := filepath.Clean(entry)
	if filepath.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, "..") {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping unsafe %s entry", configKey), fieldKey, entry)
		return "", "", false, true
	}

	srcPath := filepath.Join(repoBase, cleaned)
	dstPath = filepath.Join(wtBase, cleaned)
	if !IsPathWithinBase(srcPath, repoBase) || !IsPathWithinBase(dstPath, wtBase) {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry escaping base directory", configKey), fieldKey, entry)
		return "", "", false, true
	}

	var resolveSrcErr error
	resolvedSrc, resolveSrcErr = filepath.EvalSymlinks(srcPath)
	if resolveSrcErr != nil {
		if !errors.Is(resolveSrcErr, os.ErrNotExist) {
			slog.Warn("[WARN-GIT] failed to resolve source symlink for copy",
				"src", srcPath, "error", resolveSrcErr)
			return "", "", false, true
		}
		// Source does not exist — silent skip for optional entries.
		return "", "", false, false
	}
	if !IsPathWithinBase(resolvedSrc, repoBase) {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry escaping repository via symlink", configKey),
			fieldKey, entry, "resolvedSrc", resolvedSrc)
		return "", "", false, true
	}
	return resolvedSrc, dstPath, true, false
}

func (s *Service) copyConfigFileToWorktree(repoBase, wtBase, file string) bool {
	resolvedSrc, dst, canProcess, failed := validateAndResolveSourceEntry(
		repoBase, wtBase, file, "copy_files", "file",
	)
	if failed {
		return true
	}
	if !canProcess {
		return false
	}

	canWrite, failed := validateCopyDestination(dst, wtBase, file, "copy_files", "file")
	if failed {
		return true
	}
	if !canWrite {
		return false
	}

	// Note: a TOCTOU window exists between destination validation and file open.
	// This is acceptable because copy paths come from trusted local configuration.
	if copyErr := s.copyFileByStreaming(resolvedSrc, dst); copyErr != nil {
		if errors.Is(copyErr, os.ErrNotExist) {
			slog.Debug("[DEBUG-GIT] source file disappeared before copy_files stream copy, skipping",
				"src", resolvedSrc, "dst", dst)
			return false
		}
		slog.Warn("[WARN-GIT] failed to copy file to worktree",
			"src", resolvedSrc, "dst", dst, "error", copyErr)
		return true
	}
	return false
}

func validateCopyDestination(dst, wtBase, entry, configKey, fieldKey string) (canWrite bool, failed bool) {
	if dstDir := filepath.Dir(dst); dstDir != "." {
		if !ensureDirWithinBase(dstDir, wtBase, entry, configKey, fieldKey) {
			return false, true
		}
	}

	if info, lstatErr := os.Lstat(dst); lstatErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolvedDst, resolveDstErr := filepath.EvalSymlinks(dst)
			if resolveDstErr != nil {
				slog.Warn("[WARN-GIT] failed to resolve destination file symlink for copy",
					"dst", dst, "error", resolveDstErr)
				return false, true
			}
			if !IsPathWithinBase(resolvedDst, wtBase) {
				slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry writing outside worktree via symlink", configKey),
					fieldKey, entry, "resolvedDst", resolvedDst)
				return false, true
			}
			return true, false
		}
		if info.IsDir() {
			slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry because destination is an existing directory", configKey),
				fieldKey, entry, "dst", dst)
			return false, true
		}
		if !info.Mode().IsRegular() {
			slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry because destination is not a regular file", configKey),
				fieldKey, entry, "dst", dst, "mode", info.Mode())
			return false, true
		}
		slog.Warn(fmt.Sprintf("[WARN-GIT] overwriting existing destination file from %s", configKey),
			fieldKey, entry, "dst", dst)
	} else if !errors.Is(lstatErr, os.ErrNotExist) {
		slog.Warn("[WARN-GIT] failed to inspect destination file before copy",
			"dst", dst, "error", lstatErr)
		return false, true
	}
	return true, false
}

func ensureDirWithinBase(dirPath, basePath, entry, configKey, fieldKey string) bool {
	if mkErr := os.MkdirAll(dirPath, 0o755); mkErr != nil {
		slog.Warn("[WARN-GIT] failed to create destination directory",
			"dir", dirPath, "error", mkErr)
		return false
	}
	resolvedDir, resolveDirErr := filepath.EvalSymlinks(dirPath)
	if resolveDirErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve destination directory symlink for copy",
			"dir", dirPath, "error", resolveDirErr)
		return false
	}
	if !IsPathWithinBase(resolvedDir, basePath) {
		slog.Warn(fmt.Sprintf("[WARN-GIT] skipping %s entry escaping worktree via symlink", configKey),
			fieldKey, entry, "resolvedDstDir", resolvedDir)
		return false
	}
	return true
}

func normalizeCopyFailures(files []string) []string {
	var failures []string
	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		failures = append(failures, trimmed)
	}
	return failures
}

func resolveSymlinkEvaluatedBasePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink path: %w", err)
	}
	return filepath.Clean(resolvedPath), nil
}

// IsPathWithinBase returns true if path is within the base directory.
// Exported for reuse by devpanel service deps wiring.
func IsPathWithinBase(path, base string) bool {
	relPath, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// CopyConfigDirsToWorktree copies configured directories from the
// repository root to the worktree. Returns a list of dirs that failed to copy.
// Missing source directories are silently skipped (common for optional directories).
func (s *Service) CopyConfigDirsToWorktree(repoPath, wtPath string, dirs []string) []string {
	sharedBudget := &copyWalkBudget{}
	return s.copyConfigEntriesToWorktree(
		repoPath,
		wtPath,
		dirs,
		"directory",
		func(repoBase, wtBase, dir string) bool {
			return s.copyConfigDirToWorktreeWithBudget(repoBase, wtBase, dir, sharedBudget)
		},
	)
}

func (s *Service) copyConfigDirToWorktreeWithBudget(repoBase, wtBase, dir string, budget *copyWalkBudget) bool {
	if budget == nil {
		// Defensive fallback for direct unit tests and future callers.
		budget = &copyWalkBudget{}
	}

	resolvedSrc, dstDir, canProcess, failed := validateAndResolveSourceEntry(
		repoBase, wtBase, dir, "copy_dirs", "dir",
	)
	if failed {
		return true
	}
	if !canProcess {
		return false
	}

	// Verify source is actually a directory.
	srcInfo, statErr := s.deps.Copy.StatFileInfo(resolvedSrc)
	if statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			slog.Warn("[WARN-GIT] failed to stat source directory for copy",
				"src", resolvedSrc, "error", statErr)
			return true
		}
		return false
	}
	if !srcInfo.IsDir() {
		// Entry points to a regular file, not a directory — skip silently.
		slog.Debug("[DEBUG-GIT] copy_dirs entry is not a directory, skipping",
			"dir", dir, "src", resolvedSrc)
		return false
	}

	// hadError tracks whether any error occurred during walk (monotonically set to true).
	hadError := false
	walkErr := s.deps.Copy.WalkDir(resolvedSrc, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("[WARN-GIT] walk error in copy_dirs",
				"path", path, "error", err)
			hadError = true
			// Continue walking remaining entries.
			return nil
		}

		// Compute relative path from resolved source root.
		relPath, relErr := filepath.Rel(resolvedSrc, path)
		if relErr != nil {
			slog.Warn("[WARN-GIT] failed to compute relative path in copy_dirs",
				"path", path, "error", relErr)
			hadError = true
			return nil
		}

		targetPath := filepath.Join(dstDir, relPath)

		// SECURITY: validate target stays within worktree.
		if !IsPathWithinBase(targetPath, wtBase) {
			slog.Warn("[WARN-GIT] skipping copy_dirs entry escaping worktree",
				"dir", dir, "targetPath", targetPath)
			hadError = true
			return nil
		}

		// Handle symlinks: resolve and check containment.
		if d.Type()&os.ModeSymlink != 0 {
			return s.handleSymlinkInWalk(path, targetPath, repoBase, wtBase, dir, &hadError, budget)
		}

		if d.IsDir() {
			if !ensureDirWithinBase(targetPath, wtBase, dir, "copy_dirs", "dir") {
				hadError = true
			}
			return nil
		}

		if !d.Type().IsRegular() {
			// Skip special files (devices, sockets, etc.).
			slog.Debug("[DEBUG-GIT] skipping non-regular file in copy_dirs",
				"path", path, "type", d.Type())
			return nil
		}

		fileInfo, infoErr := d.Info()
		if infoErr != nil {
			slog.Warn("[WARN-GIT] failed to read file metadata in copy_dirs",
				"path", path, "error", infoErr)
			hadError = true
			return nil
		}
		canCopy, budgetErr := s.reserveCopyWalkBudget(budget, fileInfo.Size(), dir, path, &hadError)
		if budgetErr != nil {
			return budgetErr
		}
		if !canCopy {
			return nil
		}

		return s.copyFileInWalk(path, targetPath, wtBase, dir, &hadError)
	})

	if walkErr != nil {
		slog.Warn("[WARN-GIT] directory walk failed in copy_dirs",
			"dir", dir, "error", walkErr)
		return true
	}
	return hadError
}

// handleSymlinkInWalk resolves a symlink encountered during directory walk,
// validates containment, and copies the target content.
func (s *Service) handleSymlinkInWalk(path, targetPath, repoBase, wtBase, dirEntry string, hadError *bool, budget *copyWalkBudget) error {
	if budget == nil {
		slog.Warn("[WARN-GIT] missing budget in copy_dirs symlink handling", "path", path)
		*hadError = true
		return nil
	}
	resolvedLink, linkErr := filepath.EvalSymlinks(path)
	if linkErr != nil {
		slog.Warn("[WARN-GIT] failed to resolve symlink in copy_dirs",
			"path", path, "error", linkErr)
		*hadError = true
		return nil
	}
	if !IsPathWithinBase(resolvedLink, repoBase) {
		slog.Debug("[DEBUG-GIT] skipping symlink escaping repository in copy_dirs",
			"path", path, "resolvedLink", resolvedLink)
		// Skip without counting as failure — intentional symlink outside repo.
		return nil
	}
	linkInfo, linkStatErr := s.deps.Copy.StatFileInfo(resolvedLink)
	if linkStatErr != nil {
		slog.Warn("[WARN-GIT] failed to stat resolved symlink in copy_dirs",
			"path", resolvedLink, "error", linkStatErr)
		*hadError = true
		return nil
	}
	if linkInfo.IsDir() {
		// Create directory in worktree for symlinked directory.
		// NOTE: Contents of the symlinked directory are intentionally NOT recursed.
		// filepath.WalkDir does not follow symlinks, so this creates an empty
		// directory shell. This is a safety measure to prevent infinite loops
		// from circular symlinks and unexpected data exposure.
		if !ensureDirWithinBase(targetPath, wtBase, dirEntry, "copy_dirs", "dir") {
			*hadError = true
		} else {
			slog.Info("[INFO-GIT] created empty directory shell for symlinked directory in copy_dirs",
				"path", path, "resolvedLink", resolvedLink, "targetPath", targetPath)
		}
		return nil
	}
	if !linkInfo.Mode().IsRegular() {
		slog.Debug("[DEBUG-GIT] skipping non-regular symlink target in copy_dirs",
			"path", resolvedLink, "mode", linkInfo.Mode())
		return nil
	}
	// Symlink points to a regular file — copy it.
	canCopy, budgetErr := s.reserveCopyWalkBudget(budget, linkInfo.Size(), dirEntry, resolvedLink, hadError)
	if budgetErr != nil {
		return budgetErr
	}
	if !canCopy {
		return nil
	}
	return s.copyFileInWalk(resolvedLink, targetPath, wtBase, dirEntry, hadError)
}

// copyFileInWalk copies a single file during directory walk.
// Updates hadError on failure. Returns nil to continue walking.
func (s *Service) copyFileInWalk(srcPath, dstPath, wtBase, dirEntry string, hadError *bool) error {
	// Note: a TOCTOU window exists between destination validation and file open.
	// This is acceptable because copy paths come from trusted local configuration.
	canWrite, failed := validateCopyDestination(dstPath, wtBase, dirEntry, "copy_dirs", "dir")
	if failed {
		*hadError = true
		return nil
	}
	if !canWrite {
		return nil
	}

	if copyErr := s.copyFileByStreaming(srcPath, dstPath); copyErr != nil {
		if errors.Is(copyErr, os.ErrNotExist) {
			slog.Debug("[DEBUG-GIT] source file disappeared during copy_dirs walk, skipping",
				"src", srcPath, "dst", dstPath)
			return nil
		}
		slog.Warn("[WARN-GIT] failed to copy file in copy_dirs",
			"src", srcPath, "dst", dstPath, "error", copyErr)
		*hadError = true
	}
	return nil
}

func (s *Service) reserveCopyWalkBudget(
	budget *copyWalkBudget,
	fileSize int64,
	dirEntry string,
	srcPath string,
	hadError *bool,
) (canCopy bool, walkErr error) {
	if fileSize < 0 {
		slog.Warn("[WARN-GIT] skipping copy_dirs entry with invalid file size",
			"dir", dirEntry, "path", srcPath, "size", fileSize)
		*hadError = true
		return false, nil
	}
	nextFileCount := budget.fileCount + 1
	if nextFileCount > s.deps.Copy.MaxCopyDirsFileCount {
		slog.Warn("[WARN-GIT] aborting copy_dirs walk due to file count limit",
			"dir", dirEntry,
			"limit", s.deps.Copy.MaxCopyDirsFileCount,
			"processedFiles", budget.fileCount)
		*hadError = true
		return false, filepath.SkipAll
	}
	if budget.totalSize > s.deps.Copy.MaxCopyDirsTotalBytes || fileSize > s.deps.Copy.MaxCopyDirsTotalBytes-budget.totalSize {
		slog.Warn("[WARN-GIT] aborting copy_dirs walk due to total size limit",
			"dir", dirEntry,
			"limitBytes", s.deps.Copy.MaxCopyDirsTotalBytes,
			"processedBytes", budget.totalSize,
			"path", srcPath,
			"nextFileSize", fileSize)
		*hadError = true
		return false, filepath.SkipAll
	}
	nextTotalSize := budget.totalSize + fileSize
	budget.fileCount = nextFileCount
	budget.totalSize = nextTotalSize
	return true, nil
}

func (s *Service) copyFileByStreaming(srcPath, dstPath string) (retErr error) {
	srcFile, openSrcErr := os.Open(srcPath)
	if openSrcErr != nil {
		if errors.Is(openSrcErr, os.ErrNotExist) {
			return openSrcErr
		}
		return fmt.Errorf("open source file: %w", openSrcErr)
	}
	defer closeFileAndJoinError(srcFile, "source file", &retErr)

	// Create destination files with owner-only permissions.
	// We intentionally do not preserve source mode bits for copied config data.
	dstFile, openDstErr := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if openDstErr != nil {
		return fmt.Errorf("open destination file: %w", openDstErr)
	}
	synced := false
	defer func() {
		if retErr == nil || synced {
			return
		}
		if removeErr := s.deps.Copy.RemoveFile(dstPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove partial destination file: %w", removeErr))
		}
	}()
	defer closeFileAndJoinError(dstFile, "destination file", &retErr)

	if _, copyErr := s.deps.Copy.StreamCopy(dstFile, srcFile); copyErr != nil {
		return fmt.Errorf("stream copy file: %w", copyErr)
	}
	if syncErr := s.deps.Copy.SyncFile(dstFile); syncErr != nil {
		return fmt.Errorf("sync destination file: %w", syncErr)
	}
	synced = true
	return nil
}

func closeFileAndJoinError(file *os.File, label string, retErr *error) {
	if file == nil {
		return
	}
	if closeErr := file.Close(); closeErr != nil {
		wrapped := fmt.Errorf("close %s: %w", label, closeErr)
		if *retErr == nil {
			*retErr = wrapped
			return
		}
		*retErr = errors.Join(*retErr, wrapped)
	}
}
