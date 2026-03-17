package main

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	gitpkg "myT-x/internal/git"
)

// validateDevPanelGitFilePath validates a relative file path for git operations.
// Rejects empty, absolute, and path-traversal paths.
func validateDevPanelGitFilePath(path string) error {
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
func (a *App) resolveAndValidateGitSession(sessionName string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", errors.New("session name is required")
	}

	workDir, err := a.resolveSessionWorkDir(sessionName)
	if err != nil {
		return "", err
	}

	if !gitpkg.IsGitRepository(workDir) {
		return "", fmt.Errorf("not a git repository: %s", workDir)
	}

	return workDir, nil
}

// DevPanelGitStage stages a file for commit (git add).
func (a *App) DevPanelGitStage(sessionName string, path string) error {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	if err := validateDevPanelGitFilePath(path); err != nil {
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

// DevPanelGitUnstage unstages a file (git restore --staged).
// For fresh repositories (no commits), uses git rm --cached instead.
func (a *App) DevPanelGitUnstage(sessionName string, path string) error {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	if err := validateDevPanelGitFilePath(path); err != nil {
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

// DevPanelGitDiscard discards working changes for a file.
// For tracked files, restores from index (git restore).
// For untracked files, removes the file (git clean -f).
func (a *App) DevPanelGitDiscard(sessionName string, path string) error {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return err
	}

	if err := validateDevPanelGitFilePath(path); err != nil {
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

// DevPanelGitStageAll stages all changes for commit (git add -A).
func (a *App) DevPanelGitStageAll(sessionName string) error {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
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

// DevPanelGitUnstageAll unstages all staged changes.
// For fresh repositories, uses git rm -r --cached.
func (a *App) DevPanelGitUnstageAll(sessionName string) error {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
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

// DevPanelGitCommit creates a commit with the currently staged changes.
// Returns the short hash and message of the created commit.
func (a *App) DevPanelGitCommit(sessionName string, message string) (DevPanelCommitResult, error) {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return DevPanelCommitResult{}, err
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return DevPanelCommitResult{}, errors.New("commit message is required")
	}

	// Verify there are staged changes.
	_, diffErr := gitpkg.RunGitCLIPublic(workDir, []string{"diff", "--cached", "--quiet"})
	if diffErr == nil {
		// Exit code 0 means no staged changes.
		return DevPanelCommitResult{}, errors.New("no staged changes to commit")
	}

	// Commit.
	_, commitErr := gitpkg.RunGitCLIPublic(workDir, []string{"commit", "-m", message})
	if commitErr != nil {
		return DevPanelCommitResult{}, fmt.Errorf("git commit failed: %w", commitErr)
	}

	// Get the short hash of the created commit.
	hashOutput, hashErr := gitpkg.RunGitCLIPublic(workDir, []string{"rev-parse", "--short", "HEAD"})
	if hashErr != nil {
		slog.Warn("[DEVPANEL-GIT] commit succeeded but failed to read hash", "error", hashErr)
		return DevPanelCommitResult{Message: firstLine(message)}, nil
	}

	hash := strings.TrimSpace(string(hashOutput))
	slog.Debug("[DEVPANEL-GIT] committed", "session", sessionName, "hash", hash)

	return DevPanelCommitResult{
		Hash:    hash,
		Message: firstLine(message),
	}, nil
}

// DevPanelGitPush pushes the current branch to its remote.
// If no upstream is set, automatically sets it with -u.
func (a *App) DevPanelGitPush(sessionName string) (DevPanelPushResult, error) {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return DevPanelPushResult{}, err
	}

	// Get current branch name.
	branchOutput, branchErr := gitpkg.RunGitCLIPublic(workDir, []string{"rev-parse", "--abbrev-ref", "HEAD"})
	if branchErr != nil {
		return DevPanelPushResult{}, fmt.Errorf("failed to determine current branch: %w", branchErr)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Try push.
	_, pushErr := gitpkg.RunGitCLIPublic(workDir, []string{"push", "origin", "HEAD"})
	if pushErr == nil {
		slog.Debug("[DEVPANEL-GIT] pushed", "session", sessionName, "branch", branch)
		return DevPanelPushResult{
			RemoteName: "origin",
			BranchName: branch,
		}, nil
	}

	// Push failed — try with --set-upstream.
	slog.Debug("[DEVPANEL-GIT] push failed, retrying with -u", "error", pushErr)
	_, pushRetryErr := gitpkg.RunGitCLIPublic(workDir, []string{"push", "-u", "origin", "HEAD"})
	if pushRetryErr != nil {
		return DevPanelPushResult{}, fmt.Errorf("git push failed: %w", pushRetryErr)
	}

	slog.Debug("[DEVPANEL-GIT] pushed with upstream set", "session", sessionName, "branch", branch)
	return DevPanelPushResult{
		RemoteName:  "origin",
		BranchName:  branch,
		UpstreamSet: true,
	}, nil
}

// DevPanelGitPull pulls changes from the remote for the current branch.
func (a *App) DevPanelGitPull(sessionName string) (DevPanelPullResult, error) {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
	if err != nil {
		return DevPanelPullResult{}, err
	}

	output, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"pull"})
	if gitErr != nil {
		return DevPanelPullResult{}, fmt.Errorf("git pull failed: %w", gitErr)
	}

	summary := strings.TrimSpace(string(output))
	updated := !strings.Contains(summary, "Already up to date")

	slog.Debug("[DEVPANEL-GIT] pulled", "session", sessionName, "updated", updated)
	return DevPanelPullResult{
		Updated: updated,
		Summary: summary,
	}, nil
}

// DevPanelGitFetch fetches from all remotes and prunes deleted references.
func (a *App) DevPanelGitFetch(sessionName string) error {
	workDir, err := a.resolveAndValidateGitSession(sessionName)
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

// firstLine returns the first line of a multiline string.
func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}
