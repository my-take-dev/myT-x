package git

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"myT-x/internal/procutil"
)

// Git command retry settings for handling index.lock conflicts.
// Uses exponential backoff: 100ms, 200ms, 400ms, ... capped at 1600ms.
const (
	maxGitRetries        = 10
	gitRetryBaseInterval = 100 * time.Millisecond
	gitRetryMaxInterval  = 1600 * time.Millisecond
	// Maximum number of concurrent git commands.
	// Set to 4 to balance parallelism against git index.lock contention;
	// higher values increase lock conflicts on the same repository.
	maxConcurrentGitCommands = 4
	// Timeout for acquiring the git semaphore. Prevents indefinite blocking
	// when all semaphore slots are occupied by long-running git operations.
	semaphoreAcquireTimeout = 30 * time.Second
)

// gitSemaphore limits the number of concurrent git command executions.
var gitSemaphore = make(chan struct{}, maxConcurrentGitCommands)

func acquireGitSemaphore() error {
	select {
	case gitSemaphore <- struct{}{}:
		return nil
	case <-time.After(semaphoreAcquireTimeout):
		return fmt.Errorf("git semaphore acquisition timed out after %v", semaphoreAcquireTimeout)
	}
}

func releaseGitSemaphore() {
	<-gitSemaphore
}

// isLockFileConflict checks if the error indicates a git lock file conflict.
// Matches both "index.lock" and generic "Unable to create... File exists" messages
// (e.g., shallow.lock, pack-refs.lock).
func isLockFileConflict(errMsg string) bool {
	return strings.Contains(errMsg, "index.lock") ||
		(strings.Contains(errMsg, "Unable to create") && strings.Contains(errMsg, "File exists"))
}

// runGitCLI is the shared implementation for running git commands.
// Handles semaphore concurrency limiting, index.lock retry, and Windows console-window suppression.
// SECURITY: executes only "git" binary with application-constructed args (not user input).
func runGitCLI(dir string, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("git: no command specified")
	}

	start := time.Now()
	defer func() {
		// NOTE: git args are application-constructed (branch names, paths, flags);
		// no credentials or secrets are passed via args in this codebase.
		slog.Debug("[DEBUG-GIT] git command completed",
			"dir", dir,
			"args", args,
			"duration_ms", time.Since(start).Milliseconds())
	}()

	if err := acquireGitSemaphore(); err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	defer releaseGitSemaphore()

	var lastErrMsg string

	for attempt := 0; attempt < maxGitRetries; attempt++ {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		procutil.HideWindow(cmd)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			return stdout.Bytes(), nil
		}

		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		lastErrMsg = errMsg

		if !isLockFileConflict(errMsg) {
			return nil, fmt.Errorf("git %s failed: %s", args[0], strings.TrimSpace(errMsg))
		}

		if attempt < maxGitRetries-1 {
			backoff := gitRetryBaseInterval << uint(attempt)
			if backoff > gitRetryMaxInterval {
				backoff = gitRetryMaxInterval
			}
			slog.Debug("[DEBUG-GIT] lock file conflict, retrying",
				"attempt", attempt+1, "maxRetries", maxGitRetries,
				"backoff_ms", backoff.Milliseconds(), "args", args,
				"dir", dir)
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("git %s failed after %d retries (lock file conflict): %s",
		args[0], maxGitRetries, strings.TrimSpace(lastErrMsg))
}

// executeGitCommand runs a git command bound to the repository directory.
func (r *Repository) executeGitCommand(args []string) ([]byte, error) {
	return runGitCLI(r.path, args)
}

// runGitCommand executes a git command and returns trimmed output.
func (r *Repository) runGitCommand(args ...string) (string, error) {
	output, err := r.executeGitCommand(args)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// runGitCommandRaw executes a git command and returns output with only trailing newlines trimmed.
func (r *Repository) runGitCommandRaw(args ...string) (string, error) {
	output, err := r.executeGitCommand(args)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(output), "\n\r"), nil
}

// executeGitCommandAt runs a git command at a specific directory (not bound to repository).
func executeGitCommandAt(dir string, args []string) ([]byte, error) {
	return runGitCLI(dir, args)
}
