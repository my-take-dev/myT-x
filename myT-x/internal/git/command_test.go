package git

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestIsLockFileConflict(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{
			name:   "index.lock message",
			errMsg: "fatal: Unable to create '/repo/.git/index.lock': File exists.",
			want:   true,
		},
		{
			name:   "index.lock substring only",
			errMsg: "Another git process seems to be running; index.lock exists",
			want:   true,
		},
		{
			name:   "Unable to create + File exists without index.lock",
			errMsg: "fatal: Unable to create '/repo/.git/shallow.lock': File exists",
			want:   true,
		},
		{
			name:   "unrelated error",
			errMsg: "fatal: not a git repository",
			want:   false,
		},
		{
			name:   "empty string",
			errMsg: "",
			want:   false,
		},
		{
			name:   "Unable to create without File exists",
			errMsg: "Unable to create directory: permission denied",
			want:   false,
		},
		{
			name:   "File exists without Unable to create",
			errMsg: "File exists: /tmp/something",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLockFileConflict(tt.errMsg); got != tt.want {
				t.Fatalf("isLockFileConflict(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestUpsertEnvVar(t *testing.T) {
	t.Run("updates existing key with platform-specific key matching", func(t *testing.T) {
		env := []string{"Path=C:\\tools", "lang=ja_JP.UTF-8"}
		got := upsertEnvVar(env, "LANG", "C")
		if runtime.GOOS == "windows" {
			if got[1] != "LANG=C" {
				t.Fatalf("updated env entry = %q, want LANG=C", got[1])
			}
			return
		}
		if len(got) != 3 {
			t.Fatalf("env length = %d, want 3 on case-sensitive platforms (env=%v)", len(got), got)
		}
		if got[1] != "lang=ja_JP.UTF-8" || got[2] != "LANG=C" {
			t.Fatalf("unexpected env entries on case-sensitive platforms: %v", got)
		}
	})

	t.Run("appends missing key", func(t *testing.T) {
		env := []string{"Path=C:\\tools"}
		got := upsertEnvVar(env, "LC_ALL", "C")
		found := slices.Contains(got, "LC_ALL=C")
		if !found {
			t.Fatalf("LC_ALL=C not found in env: %v", got)
		}
	})
}

func TestLocaleNeutralGitEnv(t *testing.T) {
	env := []string{
		"Path=C:\\tools",
		"LANG=ja_JP.UTF-8",
		"LC_MESSAGES=ja_JP.UTF-8",
	}
	got := localeNeutralGitEnv(env)

	expectContains := []string{"LC_ALL=C", "LC_MESSAGES=C", "LANG=C"}
	for _, expected := range expectContains {
		found := slices.Contains(got, expected)
		if !found {
			t.Fatalf("expected %q in env, got %v", expected, got)
		}
	}

	langCount := 0
	for _, entry := range got {
		if strings.HasPrefix(entry, "LANG=") {
			langCount++
		}
	}
	if langCount != 1 {
		t.Fatalf("LANG entries = %d, want 1 (env=%v)", langCount, got)
	}
}

func TestAcquireGitSemaphoreWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := acquireGitSemaphoreWithContext(ctx)
	if err == nil {
		t.Fatal("acquireGitSemaphoreWithContext() expected cancellation error")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("error = %v, want canceled", err)
	}
}

func TestRunGitCLIWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runGitCLIWithContext(ctx, ".", []string{"status"})
	if err == nil {
		t.Fatal("runGitCLIWithContext() expected cancellation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "canceled") {
		t.Fatalf("error = %v, want canceled message", err)
	}
}

func TestRunGitCLIWithContextAndDepsRejectsEmptyArgs(t *testing.T) {
	_, err := runGitCLIWithContextAndDeps(
		context.Background(),
		".",
		nil,
		[]string{"PATH=dummy"},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("runGitCLIWithContextAndDeps() expected empty-args validation error")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Fatalf("error = %v, want no command specified", err)
	}
}

func TestGitRetryBackoff(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "negative attempt clamps to base", attempt: -1, want: gitRetryBaseInterval},
		{name: "first retry", attempt: 0, want: gitRetryBaseInterval},
		{name: "second retry", attempt: 1, want: 2 * gitRetryBaseInterval},
		{name: "cap at max", attempt: 10, want: gitRetryMaxInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitRetryBackoff(tt.attempt); got != tt.want {
				t.Fatalf("gitRetryBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestRunGitCLIWithContextAndDepsRetriesLockConflicts(t *testing.T) {
	attempts := 0
	waits := make([]time.Duration, 0, 2)
	runner := func(_ context.Context, _ string, _ []string, _ []string) ([]byte, string, error) {
		attempts++
		if attempts < 3 {
			return nil, "fatal: Unable to create '/repo/.git/index.lock': File exists.", errors.New("exit status 1")
		}
		return []byte("ok\n"), "", nil
	}
	waiter := func(_ context.Context, backoff time.Duration) error {
		waits = append(waits, backoff)
		return nil
	}

	out, err := runGitCLIWithContextAndDeps(
		context.Background(),
		".",
		[]string{"status"},
		[]string{"PATH=dummy"},
		runner,
		waiter,
	)
	if err != nil {
		t.Fatalf("runGitCLIWithContextAndDeps() error = %v, want nil", err)
	}
	if string(out) != "ok\n" {
		t.Fatalf("output = %q, want %q", string(out), "ok\n")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if len(waits) != 2 || waits[0] != gitRetryBaseInterval || waits[1] != 2*gitRetryBaseInterval {
		t.Fatalf("wait backoffs = %v, want [%v %v]", waits, gitRetryBaseInterval, 2*gitRetryBaseInterval)
	}
}

func TestRunGitCLIWithContextAndDepsDoesNotRetryNonLockErrors(t *testing.T) {
	attempts := 0
	waitCalls := 0
	runner := func(_ context.Context, _ string, _ []string, _ []string) ([]byte, string, error) {
		attempts++
		return nil, "fatal: not a git repository", errors.New("exit status 128")
	}
	waiter := func(_ context.Context, _ time.Duration) error {
		waitCalls++
		return nil
	}

	_, err := runGitCLIWithContextAndDeps(
		context.Background(),
		".",
		[]string{"status"},
		[]string{"PATH=dummy"},
		runner,
		waiter,
	)
	if err == nil {
		t.Fatal("runGitCLIWithContextAndDeps() expected non-lock failure")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %v, want repository message", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if waitCalls != 0 {
		t.Fatalf("wait calls = %d, want 0", waitCalls)
	}
}

func TestRunGitCLIWithContextAndDepsExhaustsRetriesOnLockConflict(t *testing.T) {
	attempts := 0
	waitCalls := 0
	runner := func(_ context.Context, _ string, _ []string, _ []string) ([]byte, string, error) {
		attempts++
		return nil, "fatal: Unable to create '/repo/.git/index.lock': File exists.", errors.New("exit status 1")
	}
	waiter := func(_ context.Context, _ time.Duration) error {
		waitCalls++
		return nil
	}

	_, err := runGitCLIWithContextAndDeps(
		context.Background(),
		".",
		[]string{"status"},
		[]string{"PATH=dummy"},
		runner,
		waiter,
	)
	if err == nil {
		t.Fatal("runGitCLIWithContextAndDeps() expected retry exhaustion error")
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Fatalf("error = %v, want retry exhaustion message", err)
	}
	if attempts != maxGitRetries {
		t.Fatalf("attempts = %d, want %d", attempts, maxGitRetries)
	}
	if waitCalls != maxGitRetries-1 {
		t.Fatalf("wait calls = %d, want %d", waitCalls, maxGitRetries-1)
	}
}

func TestRunGitCLIWithContextAndDepsStopsWhenBackoffWaitFails(t *testing.T) {
	attempts := 0
	waitCalls := 0
	runner := func(_ context.Context, _ string, _ []string, _ []string) ([]byte, string, error) {
		attempts++
		return nil, "fatal: Unable to create '/repo/.git/index.lock': File exists.", errors.New("exit status 1")
	}
	waiter := func(_ context.Context, _ time.Duration) error {
		waitCalls++
		return context.Canceled
	}

	_, err := runGitCLIWithContextAndDeps(
		context.Background(),
		".",
		[]string{"status"},
		[]string{"PATH=dummy"},
		runner,
		waiter,
	)
	if err == nil {
		t.Fatal("runGitCLIWithContextAndDeps() expected canceled during retry backoff error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "canceled during retry backoff") {
		t.Fatalf("error = %v, want canceled-during-backoff message", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if waitCalls != 1 {
		t.Fatalf("wait calls = %d, want 1", waitCalls)
	}
}
