package git

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
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

type gitCommandRunner func(ctx context.Context, dir string, args []string, env []string) ([]byte, string, error)
type gitRetryWaiter func(ctx context.Context, backoff time.Duration) error

// gitSemaphore limits the number of concurrent git command executions.
// NOTE: This limiter is process-wide, not per-repository. Different repositories
// can block each other under heavy concurrent git traffic.
// TODO: Consider per-repository semaphores if this becomes a bottleneck.
var gitSemaphore = make(chan struct{}, maxConcurrentGitCommands)

func acquireGitSemaphoreWithContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("git semaphore acquisition canceled: %w", err)
	}
	timer := time.NewTimer(semaphoreAcquireTimeout)
	defer timer.Stop()

	select {
	case gitSemaphore <- struct{}{}:
		return nil
	case <-timer.C:
		return fmt.Errorf("git semaphore acquisition timed out after %v", semaphoreAcquireTimeout)
	case <-ctx.Done():
		return fmt.Errorf("git semaphore acquisition canceled: %w", ctx.Err())
	}
}

func releaseGitSemaphore() {
	<-gitSemaphore
}

func localeNeutralGitEnv(baseEnv []string) []string {
	env := append([]string(nil), baseEnv...)
	env = upsertEnvVar(env, "LC_ALL", "C")
	env = upsertEnvVar(env, "LC_MESSAGES", "C")
	env = upsertEnvVar(env, "LANG", "C")
	return env
}

func upsertEnvVar(env []string, key, value string) []string {
	for i := range env {
		parts := strings.SplitN(env[i], "=", 2)
		if len(parts) != 2 {
			continue
		}
		if isEnvVarKeyMatch(parts[0], key) {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
}

func isEnvVarKeyMatch(existingKey, targetKey string) bool {
	// Windows treats environment variable names case-insensitively.
	if runtime.GOOS == "windows" {
		return strings.EqualFold(existingKey, targetKey)
	}
	return existingKey == targetKey
}

// isLockFileConflict checks if the error indicates a git lock file conflict.
// Matches both "index.lock" and generic "Unable to create... File exists" messages
// (e.g., shallow.lock, pack-refs.lock).
func isLockFileConflict(errMsg string) bool {
	return strings.Contains(errMsg, "index.lock") ||
		(strings.Contains(errMsg, "Unable to create") && strings.Contains(errMsg, "File exists"))
}

func gitRetryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	backoff := gitRetryBaseInterval << uint(attempt)
	if backoff > gitRetryMaxInterval {
		return gitRetryMaxInterval
	}
	return backoff
}

func defaultGitCommandRunner(ctx context.Context, dir string, args []string, env []string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = env
	procutil.HideWindow(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

func waitForGitRetryBackoff(ctx context.Context, backoff time.Duration) error {
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runGitCLIWithContextAndDeps(
	ctx context.Context,
	dir string,
	args []string,
	env []string,
	runner gitCommandRunner,
	retryWaiter gitRetryWaiter,
) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("git: no command specified")
	}
	if runner == nil {
		runner = defaultGitCommandRunner
	}
	if retryWaiter == nil {
		retryWaiter = waitForGitRetryBackoff
	}
	if err := acquireGitSemaphoreWithContext(ctx); err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	defer releaseGitSemaphore()

	var lastErrMsg string
	for attempt := range maxGitRetries {
		stdout, stderrText, err := runner(ctx, dir, args, env)
		if err == nil {
			return stdout, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("git %s canceled: %w", args[0], ctxErr)
		}

		errMsg := stderrText
		if errMsg == "" {
			errMsg = err.Error()
		}
		lastErrMsg = errMsg

		if !isLockFileConflict(errMsg) {
			return nil, fmt.Errorf("git %s failed: %s", args[0], strings.TrimSpace(errMsg))
		}

		if attempt >= maxGitRetries-1 {
			slog.Warn("[WARN-GIT] lock file conflict retries exhausted",
				"maxRetries", maxGitRetries, "args", args, "dir", dir, "error", strings.TrimSpace(errMsg))
			continue
		}
		backoff := gitRetryBackoff(attempt)
		slog.Debug("[DEBUG-GIT] lock file conflict, retrying",
			"attempt", attempt+1, "maxRetries", maxGitRetries,
			"backoff_ms", backoff.Milliseconds(), "args", args,
			"dir", dir)
		if waitErr := retryWaiter(ctx, backoff); waitErr != nil {
			return nil, fmt.Errorf("git %s canceled during retry backoff: %w", args[0], waitErr)
		}
	}

	return nil, fmt.Errorf("git %s failed after %d retries (lock file conflict): %s",
		args[0], maxGitRetries, strings.TrimSpace(lastErrMsg))
}

// runGitCLI is the shared implementation for running git commands.
// Handles semaphore concurrency limiting, index.lock retry, and Windows console-window suppression.
// SECURITY: executes only "git" binary with application-constructed args (not user input).
func runGitCLI(dir string, args []string) ([]byte, error) {
	return runGitCLIWithContext(context.Background(), dir, args)
}

// runGitCLIWithContext runs git commands with cancellation support via context.
func runGitCLIWithContext(ctx context.Context, dir string, args []string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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

	env := localeNeutralGitEnv(os.Environ())
	return runGitCLIWithContextAndDeps(
		ctx,
		dir,
		args,
		env,
		defaultGitCommandRunner,
		waitForGitRetryBackoff,
	)
}

// executeGitCommand runs a git command bound to the repository directory.
func (r *Repository) executeGitCommand(args []string) ([]byte, error) {
	return r.executeGitCommandWithContext(context.Background(), args)
}

// executeGitCommandWithContext runs a git command bound to the repository directory with context.
func (r *Repository) executeGitCommandWithContext(ctx context.Context, args []string) ([]byte, error) {
	return runGitCLIWithContext(ctx, r.path, args)
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

// RunGitCLIPublic is a public wrapper around runGitCLI for use by app-layer
// code that needs to run git commands without opening a Repository first.
// SECURITY: executes only "git" binary with application-constructed args.
func RunGitCLIPublic(dir string, args []string) ([]byte, error) {
	return runGitCLI(dir, args)
}
