package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/testutil"
)

func TestIsGitRepository(t *testing.T) {
	testutil.SkipIfNoGit(t)

	t.Run("valid git repo", func(t *testing.T) {
		dir := testutil.CreateTempGitRepo(t)
		if !IsGitRepository(dir) {
			t.Error("expected IsGitRepository to return true for a git repo")
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		dir := t.TempDir()
		if IsGitRepository(dir) {
			t.Error("expected IsGitRepository to return false for a non-git directory")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		if IsGitRepository("/nonexistent/path/12345") {
			t.Error("expected IsGitRepository to return false for nonexistent path")
		}
	})
}

func TestOpen(t *testing.T) {
	testutil.SkipIfNoGit(t)

	t.Run("valid repo", func(t *testing.T) {
		dir := testutil.CreateTempGitRepo(t)
		repo, err := Open(dir)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if repo.GetPath() != dir {
			t.Errorf("GetPath() = %q, want %q", repo.GetPath(), dir)
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Open(dir)
		if err == nil {
			t.Error("expected Open() to return error for non-git directory")
		}
	})
}

func TestFindRepoRoot(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	subDir := filepath.Join(dir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRepoRoot(subDir)
	if err != nil {
		t.Fatalf("FindRepoRoot() error = %v", err)
	}
	// Normalize paths for comparison (resolve symlinks on some OS).
	wantAbs := testutil.ResolvePath(dir)
	gotAbs := testutil.ResolvePath(root)
	if gotAbs != wantAbs {
		t.Errorf("FindRepoRoot() = %q, want %q", gotAbs, wantAbs)
	}
}

func TestCurrentBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	branch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	// Default branch could be "main" or "master" depending on git config.
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestListBranches(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(branches) == 0 {
		t.Error("expected at least one branch")
	}
}

func TestCheckoutNewBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a detached worktree.
	wtDir := GenerateWorktreeDirPath(dir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := GenerateWorktreePath(dir, "test-promote")
	if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
		t.Fatalf("CreateWorktreeDetached() error = %v", err)
	}

	// Open the worktree as a repository.
	wtRepo, err := Open(wtPath)
	if err != nil {
		t.Fatalf("Open(worktree) error = %v", err)
	}

	// Verify detached HEAD.
	branch, err := wtRepo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "" {
		t.Errorf("expected detached HEAD (empty branch), got %q", branch)
	}

	// Promote to branch.
	if err := wtRepo.CheckoutNewBranch("feature/promoted"); err != nil {
		t.Fatalf("CheckoutNewBranch() error = %v", err)
	}

	// Verify branch name.
	branch, err = wtRepo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() after promote error = %v", err)
	}
	if branch != "feature/promoted" {
		t.Errorf("CurrentBranch() = %q, want %q", branch, "feature/promoted")
	}
}

func TestCheckoutNewBranchValidation(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		branchName string
	}{
		{"empty", ""},
		{"starts with dash", "-bad"},
		{"starts with dot", ".hidden"},
		{"contains dotdot", "a..b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.CheckoutNewBranch(tc.branchName)
			if err == nil {
				t.Errorf("CheckoutNewBranch(%q) expected error, got nil", tc.branchName)
			}
		})
	}
}

// createBareAndClone creates a bare repo and a clone for push/pull testing.
func createBareAndClone(t *testing.T) (bareDir, cloneDir string) {
	t.Helper()
	testutil.SkipIfNoGit(t)

	bareDir = testutil.ResolvePath(t.TempDir())
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

	cloneDir = testutil.ResolvePath(t.TempDir())
	cmd = exec.Command("git", "clone", bareDir, ".")
	cmd.Dir = cloneDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}

	for _, args := range [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Dir = cloneDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Initial commit and push so upstream tracking is set up.
	if err := os.WriteFile(filepath.Join(cloneDir, "develop-README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
		{"git", "push", "origin", "HEAD"},
	} {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Dir = cloneDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	return bareDir, cloneDir
}

func TestCommitAll(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// CommitAll should stage and commit.
	if err := repo.CommitAll("add new file"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Verify no uncommitted changes remain.
	has, err := repo.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error = %v", err)
	}
	if has {
		t.Error("expected no uncommitted changes after CommitAll")
	}
}

func TestCommitAllEmptyMessage(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.CommitAll(""); err == nil {
		t.Error("CommitAll with empty message should return error")
	}
}

func TestPushAndPull(t *testing.T) {
	testutil.SkipIfNoGit(t)

	_, cloneDir := createBareAndClone(t)

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a file and commit.
	if err := os.WriteFile(filepath.Join(cloneDir, "feature.txt"), []byte("feature"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitAll("add feature"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Push.
	if err := repo.Push(); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	// Pull (should be no-op since we are up to date).
	if err := repo.Pull(); err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
}

func TestHasUnpushedCommits(t *testing.T) {
	testutil.SkipIfNoGit(t)

	_, cloneDir := createBareAndClone(t)

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatal(err)
	}

	// Initially no unpushed commits.
	has, err := repo.HasUnpushedCommits()
	if err != nil {
		t.Fatalf("HasUnpushedCommits() error = %v", err)
	}
	if has {
		t.Error("expected no unpushed commits initially")
	}

	// Create a local commit.
	if err := os.WriteFile(filepath.Join(cloneDir, "local.txt"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitAll("local commit"); err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Should have unpushed commits.
	has, err = repo.HasUnpushedCommits()
	if err != nil {
		t.Fatalf("HasUnpushedCommits() error = %v", err)
	}
	if !has {
		t.Error("expected unpushed commits after local commit")
	}

	// Push and verify.
	if err := repo.Push(); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	has, err = repo.HasUnpushedCommits()
	if err != nil {
		t.Fatalf("HasUnpushedCommits() after push error = %v", err)
	}
	if has {
		t.Error("expected no unpushed commits after push")
	}
}

func TestHasUnpushedCommitsNoUpstream(t *testing.T) {
	testutil.SkipIfNoGit(t)

	// Repo without remote → should return false, not error.
	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	has, err := repo.HasUnpushedCommits()
	if err != nil {
		t.Fatalf("HasUnpushedCommits() error = %v", err)
	}
	if has {
		t.Error("expected false for repo without upstream")
	}
}

func TestHasUnpushedCommitsDetachedHEAD(t *testing.T) {
	testutil.SkipIfNoGit(t)

	_, cloneDir := createBareAndClone(t)

	// Detach HEAD so @{upstream} reference becomes invalid.
	cmd := exec.Command("git", "checkout", "--detach")
	cmd.Dir = cloneDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --detach failed: %v\n%s", err, out)
	}

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatal(err)
	}

	has, err := repo.HasUnpushedCommits()
	if err != nil {
		t.Fatalf("HasUnpushedCommits() unexpected error: %v", err)
	}
	if has {
		t.Error("expected false for detached HEAD")
	}
}

func TestHasUnpushedCommitsErrorPropagation(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove .git directory to cause a non-upstream error when running git commands.
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}

	_, err = repo.HasUnpushedCommits()
	if err == nil {
		t.Fatal("HasUnpushedCommits() expected error for broken repository")
	}
	// Verify the error is wrapped and propagated (not swallowed as "no upstream").
	if !strings.Contains(err.Error(), "HasUnpushedCommits") {
		t.Fatalf("error should be wrapped with HasUnpushedCommits prefix: %v", err)
	}
	// Verify %w wrapping is correct — the original error should be recoverable via Unwrap.
	if errors.Unwrap(err) == nil {
		t.Fatalf("error should wrap the original cause via %%w: %v", err)
	}
}

func TestIsNoUpstreamTrackingError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{
			name:   "no upstream configured",
			errMsg: "fatal: no upstream configured for branch 'main'",
			want:   true,
		},
		{
			name:   "detached head valid ref",
			errMsg: "fatal: '@{upstream}' is not a valid ref",
			want:   true,
		},
		{
			name:   "missing upstream ref",
			errMsg: "fatal: no such ref: 'main@{u}'",
			want:   true,
		},
		{
			name:   "head does not point to branch",
			errMsg: "fatal: HEAD does not point to a branch",
			want:   true,
		},
		{
			name:   "generic valid ref should not be swallowed",
			errMsg: "fatal: refs/heads/main is not a valid ref",
			want:   false,
		},
		{
			name:   "filesystem error should not be swallowed",
			errMsg: "fatal: cannot open .git/packed-refs: Permission denied",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoUpstreamTrackingError(tt.errMsg); got != tt.want {
				t.Fatalf("isNoUpstreamTrackingError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	testutil.SkipIfNoGit(t)

	dir := testutil.CreateTempGitRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("clean repo", func(t *testing.T) {
		has, err := repo.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("HasUncommittedChanges() error = %v", err)
		}
		if has {
			t.Error("expected no uncommitted changes in clean repo")
		}
	})

	t.Run("dirty repo", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("change"), 0o644); err != nil {
			t.Fatal(err)
		}
		has, err := repo.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("HasUncommittedChanges() error = %v", err)
		}
		if !has {
			t.Error("expected uncommitted changes")
		}
	})
}
