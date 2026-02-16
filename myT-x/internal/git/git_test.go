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

	t.Run("trims and cleans path", func(t *testing.T) {
		dir := testutil.CreateTempGitRepo(t)
		nested := filepath.Join(dir, "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		dirtyPath := "   " + nested + string(os.PathSeparator) + "..   "
		repo, err := Open(dirtyPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}

		want := filepath.Clean(dir)
		if repo.GetPath() != want {
			t.Fatalf("GetPath() = %q, want %q", repo.GetPath(), want)
		}
	})

	t.Run("stores absolute path for relative input", func(t *testing.T) {
		dir := testutil.CreateTempGitRepo(t)
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v", err)
		}
		relDir, err := filepath.Rel(cwd, dir)
		if err != nil {
			t.Skipf("skipping relative-path assertion: %v", err)
		}
		repo, err := Open(relDir)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		want, err := filepath.Abs(relDir)
		if err != nil {
			t.Fatalf("Abs() error = %v", err)
		}
		if repo.GetPath() != filepath.Clean(want) {
			t.Fatalf("GetPath() = %q, want %q", repo.GetPath(), filepath.Clean(want))
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

func TestCheckoutDetachedHead(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name            string
		setup           func(t *testing.T, repoPath string)
		wantErr         bool
		wantErrContains string
		verify          func(t *testing.T, repo *Repository, repoPath string)
	}{
		{
			name: "switches repository to detached HEAD",
			setup: func(t *testing.T, repoPath string) {
				t.Helper()
				// Ensure we start from a normal branch.
				_ = runGitCommandInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
			},
			verify: func(t *testing.T, repo *Repository, _ string) {
				t.Helper()
				branch, err := repo.CurrentBranch()
				if err != nil {
					t.Fatalf("CurrentBranch() error = %v", err)
				}
				if branch != "" {
					t.Fatalf("CurrentBranch() = %q, want detached HEAD", branch)
				}
			},
		},
		{
			name: "returns wrapped error when git command fails",
			setup: func(t *testing.T, repoPath string) {
				t.Helper()
				if err := os.RemoveAll(filepath.Join(repoPath, ".git")); err != nil {
					t.Fatalf("failed to remove .git: %v", err)
				}
			},
			wantErr:         true,
			wantErrContains: "failed to checkout detached HEAD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := testutil.CreateTempGitRepo(t)
			repo, err := Open(repoPath)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if tt.setup != nil {
				tt.setup(t, repoPath)
			}

			err = repo.CheckoutDetachedHead()
			if tt.wantErr {
				if err == nil {
					t.Fatal("CheckoutDetachedHead() expected error")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("CheckoutDetachedHead() error = %v, want substring %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("CheckoutDetachedHead() error = %v", err)
			}
			if tt.verify != nil {
				tt.verify(t, repo, repoPath)
			}
		})
	}
}

func TestDeleteLocalBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name            string
		branchName      string
		force           bool
		setup           func(t *testing.T, repoPath string)
		wantErr         bool
		wantErrContains string
		verify          func(t *testing.T, repo *Repository, repoPath string, branchName string)
	}{
		{
			name:       "deletes merged branch with -d",
			branchName: "feature/delete-normal",
			force:      false,
			setup: func(t *testing.T, repoPath string) {
				t.Helper()
				runGitCommandInDir(t, repoPath, "branch", "feature/delete-normal")
			},
			verify: func(t *testing.T, repo *Repository, _ string, branchName string) {
				t.Helper()
				assertBranchPresence(t, repo, branchName, false)
			},
		},
		{
			name:       "deletes unmerged branch with -D",
			branchName: "feature/delete-force",
			force:      true,
			setup: func(t *testing.T, repoPath string) {
				t.Helper()
				defaultBranch := runGitCommandInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, repoPath, "checkout", "-b", "feature/delete-force")
				if err := os.WriteFile(filepath.Join(repoPath, "force-delete.txt"), []byte("force"), 0o644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				runGitCommandInDir(t, repoPath, "add", ".")
				runGitCommandInDir(t, repoPath, "commit", "-m", "create unmerged commit")
				runGitCommandInDir(t, repoPath, "checkout", defaultBranch)
			},
			verify: func(t *testing.T, repo *Repository, _ string, branchName string) {
				t.Helper()
				assertBranchPresence(t, repo, branchName, false)
			},
		},
		{
			name:       "returns validation error for invalid branch name",
			branchName: "../invalid",
			force:      false,
			wantErr:    true,
		},
		{
			name:            "returns wrapped error for missing branch",
			branchName:      "feature/missing",
			force:           false,
			wantErr:         true,
			wantErrContains: `failed to delete local branch "feature/missing"`,
		},
		{
			name:       "returns wrapped error for checked out branch",
			branchName: "feature/checked-out",
			force:      false,
			setup: func(t *testing.T, repoPath string) {
				t.Helper()
				runGitCommandInDir(t, repoPath, "checkout", "-b", "feature/checked-out")
			},
			wantErr:         true,
			wantErrContains: `failed to delete local branch "feature/checked-out"`,
			verify: func(t *testing.T, repo *Repository, _ string, branchName string) {
				t.Helper()
				assertBranchPresence(t, repo, branchName, true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := testutil.CreateTempGitRepo(t)
			repo, err := Open(repoPath)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if tt.setup != nil {
				tt.setup(t, repoPath)
			}

			err = repo.DeleteLocalBranch(tt.branchName, tt.force)
			if tt.wantErr {
				if err == nil {
					t.Fatal("DeleteLocalBranch() expected error")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("DeleteLocalBranch() error = %v, want substring %q", err, tt.wantErrContains)
				}
				if tt.verify != nil {
					tt.verify(t, repo, repoPath, tt.branchName)
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteLocalBranch() error = %v", err)
			}
			if tt.verify != nil {
				tt.verify(t, repo, repoPath, tt.branchName)
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
	cmd.Env = localeNeutralGitEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

	cloneDir = testutil.ResolvePath(t.TempDir())
	cmd = exec.Command("git", "clone", bareDir, ".")
	cmd.Dir = cloneDir
	cmd.Env = localeNeutralGitEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}

	for _, args := range [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Dir = cloneDir
		cmd.Env = localeNeutralGitEnv(os.Environ())
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
		cmd.Env = localeNeutralGitEnv(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	return bareDir, cloneDir
}

func runGitCommandInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = localeNeutralGitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func assertBranchPresence(t *testing.T, repo *Repository, branchName string, wantPresent bool) {
	t.Helper()
	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	present := false
	for _, branch := range branches {
		if branch == branchName {
			present = true
			break
		}
	}
	if present != wantPresent {
		t.Fatalf("branch %q presence = %v, want %v (branches=%v)", branchName, present, wantPresent, branches)
	}
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
	if err := repo.CommitAll("   "); err == nil {
		t.Error("CommitAll with whitespace-only message should return error")
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
	cmd.Env = localeNeutralGitEnv(os.Environ())
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
			name:   "short form upstream not a valid ref",
			errMsg: "fatal: 'main@{u}' is not a valid ref",
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
			name:   "plain upstream wording should not be swallowed",
			errMsg: "fatal: 'refs/upstream/main' is not a valid ref",
			want:   false,
		},
		{
			name:   "no such ref without upstream token should not be swallowed",
			errMsg: "fatal: no such ref: 'refs/heads/nonexistent'",
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

func TestBranchTrackingInfoHasLiveUpstream(t *testing.T) {
	tests := []struct {
		name string
		info branchTrackingInfo
		want bool
	}{
		{
			name: "empty upstream",
			info: branchTrackingInfo{Upstream: "", UpstreamTrack: ""},
			want: false,
		},
		{
			name: "upstream exists without gone marker",
			info: branchTrackingInfo{Upstream: "origin/main", UpstreamTrack: ""},
			want: true,
		},
		{
			name: "gone marker",
			info: branchTrackingInfo{Upstream: "origin/main", UpstreamTrack: "[gone]"},
			want: false,
		},
		{
			name: "gone marker case insensitive",
			info: branchTrackingInfo{Upstream: "origin/main", UpstreamTrack: "[GoNe]"},
			want: false,
		},
		{
			name: "upstream track text without upstream",
			info: branchTrackingInfo{Upstream: "", UpstreamTrack: "[ahead 1]"},
			want: false,
		},
		{
			name: "upstream with ahead and behind is live",
			info: branchTrackingInfo{Upstream: "origin/main", UpstreamTrack: "[ahead 1, behind 2]"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.hasLiveUpstream(); got != tt.want {
				t.Fatalf("hasLiveUpstream() = %v, want %v (info=%+v)", got, tt.want, tt.info)
			}
		})
	}
}

func TestListBranchesForWorktreeBase(t *testing.T) {
	testutil.SkipIfNoGit(t)

	contains := func(branches []string, target string) bool {
		for _, branch := range branches {
			if branch == target {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name       string
		setup      func(t *testing.T) (*Repository, string)
		wantError  bool
		assertions func(t *testing.T, branches []string, defaultBranch string)
	}{
		{
			name: "remote branch with live upstream is included",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				defaultBranch := runGitCommandInDir(t, cloneDir, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, cloneDir, "checkout", "-b", "feature/alive")
				runGitCommandInDir(t, cloneDir, "push", "-u", "origin", "feature/alive")
				runGitCommandInDir(t, cloneDir, "checkout", defaultBranch)

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, defaultBranch
			},
			assertions: func(t *testing.T, branches []string, defaultBranch string) {
				if !contains(branches, defaultBranch) {
					t.Fatalf("default branch %q should be listed, got %v", defaultBranch, branches)
				}
				if !contains(branches, "feature/alive") {
					t.Fatalf("branch %q should be listed, got %v", "feature/alive", branches)
				}
			},
		},
		{
			name: "upstream gone branch is excluded",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				defaultBranch := runGitCommandInDir(t, cloneDir, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, cloneDir, "checkout", "-b", "feature/gone")
				runGitCommandInDir(t, cloneDir, "push", "-u", "origin", "feature/gone")
				runGitCommandInDir(t, cloneDir, "checkout", defaultBranch)
				// No fetch --prune is needed here because deleting via this same clone
				// updates its remote-tracking refs immediately.
				runGitCommandInDir(t, cloneDir, "push", "origin", "--delete", "feature/gone")

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, defaultBranch
			},
			assertions: func(t *testing.T, branches []string, defaultBranch string) {
				if !contains(branches, defaultBranch) {
					t.Fatalf("default branch %q should be listed, got %v", defaultBranch, branches)
				}
				if contains(branches, "feature/gone") {
					t.Fatalf("gone upstream branch should be excluded, got %v", branches)
				}
			},
		},
		{
			name: "fully local repository falls back to local branches",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				defaultBranch := runGitCommandInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, repoPath, "branch", "feature/local-only")

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, defaultBranch
			},
			assertions: func(t *testing.T, branches []string, defaultBranch string) {
				if !contains(branches, defaultBranch) {
					t.Fatalf("default branch %q should be listed, got %v", defaultBranch, branches)
				}
				if !contains(branches, "feature/local-only") {
					t.Fatalf("local-only branch should be listed in fallback mode, got %v", branches)
				}
			},
		},
		{
			name: "stale upstream metadata without remotes falls back to local branches",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				defaultBranch := runGitCommandInDir(t, cloneDir, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, cloneDir, "checkout", "-b", "feature/stale-upstream")
				runGitCommandInDir(t, cloneDir, "push", "-u", "origin", "feature/stale-upstream")
				runGitCommandInDir(t, cloneDir, "checkout", defaultBranch)
				runGitCommandInDir(t, cloneDir, "push", "origin", "--delete", "feature/stale-upstream")
				runGitCommandInDir(t, cloneDir, "remote", "remove", "origin")

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, defaultBranch
			},
			assertions: func(t *testing.T, branches []string, defaultBranch string) {
				if !contains(branches, defaultBranch) {
					t.Fatalf("default branch %q should be listed in local fallback mode, got %v", defaultBranch, branches)
				}
				if !contains(branches, "feature/stale-upstream") {
					t.Fatalf("stale-upstream branch should be listed in local fallback mode, got %v", branches)
				}
			},
		},
		{
			name: "branch list is empty when no local branches exist",
			setup: func(t *testing.T) (*Repository, string) {
				bareDir := testutil.ResolvePath(t.TempDir())
				runGitCommandInDir(t, bareDir, "init", "--bare", ".")

				repo, err := Open(bareDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, ""
			},
			assertions: func(t *testing.T, branches []string, _ string) {
				if branches == nil {
					t.Fatal("ListBranchesForWorktreeBase() should return an empty slice, not nil")
				}
				if len(branches) != 0 {
					t.Fatalf("expected empty branch list, got %v", branches)
				}
			},
		},
		{
			name: "local branch matching remote name is included without upstream tracking",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				defaultBranch := runGitCommandInDir(t, cloneDir, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, cloneDir, "push", "origin", "HEAD:refs/heads/feature/shared")
				runGitCommandInDir(t, cloneDir, "branch", "--no-track", "feature/shared")

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, defaultBranch
			},
			assertions: func(t *testing.T, branches []string, _ string) {
				if !contains(branches, "feature/shared") {
					t.Fatalf("remote-matched local branch should be listed, got %v", branches)
				}
			},
		},
		{
			name: "remotes exist but all local branches are stale or local-only",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				headCommit := runGitCommandInDir(t, repoPath, "rev-parse", "HEAD")
				runGitCommandInDir(t, repoPath, "update-ref", "refs/remotes/origin/remote-only", headCommit)
				runGitCommandInDir(t, repoPath, "branch", "feature/local-only")

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, ""
			},
			assertions: func(t *testing.T, branches []string, _ string) {
				// Repository has at least one remote-tracking ref, so this path is
				// intentionally not in "local-only fallback" mode.
				// Local branches without live upstream/remote match are filtered out.
				if branches == nil {
					t.Fatal("ListBranchesForWorktreeBase() should return an empty slice, not nil")
				}
				if len(branches) != 0 {
					t.Fatalf("expected empty branch list when remotes exist but local branches are stale, got %v", branches)
				}
			},
		},
		{
			name: "broken repository returns error",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				if removeErr := os.RemoveAll(filepath.Join(repoPath, ".git")); removeErr != nil {
					t.Fatalf("failed to remove .git: %v", removeErr)
				}
				return repo, ""
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, defaultBranch := tt.setup(t)
			branches, err := repo.ListBranchesForWorktreeBase()
			if tt.wantError {
				if err == nil {
					t.Fatal("ListBranchesForWorktreeBase() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ListBranchesForWorktreeBase() error = %v", err)
			}
			if tt.assertions != nil {
				tt.assertions(t, branches, defaultBranch)
			}
		})
	}
}

func TestListLocalBranchTrackingInfoParsesSlashBranchAndTrackStatus(t *testing.T) {
	testutil.SkipIfNoGit(t)

	bareDir, cloneDir := createBareAndClone(t)
	branchName := "feature/team/task-123"

	runGitCommandInDir(t, cloneDir, "checkout", "-b", branchName)
	runGitCommandInDir(t, cloneDir, "push", "-u", "origin", branchName)

	if err := os.WriteFile(filepath.Join(cloneDir, "ahead.txt"), []byte("ahead"), 0o644); err != nil {
		t.Fatalf("write ahead file: %v", err)
	}
	runGitCommandInDir(t, cloneDir, "add", "ahead.txt")
	runGitCommandInDir(t, cloneDir, "commit", "-m", "ahead commit")

	peerDir := testutil.ResolvePath(t.TempDir())
	cmd := exec.Command("git", "clone", bareDir, ".")
	cmd.Dir = peerDir
	cmd.Env = localeNeutralGitEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}
	runGitCommandInDir(t, peerDir, "config", "user.email", "test@test.com")
	runGitCommandInDir(t, peerDir, "config", "user.name", "Test")
	runGitCommandInDir(t, peerDir, "checkout", "--track", "origin/"+branchName)
	if err := os.WriteFile(filepath.Join(peerDir, "behind.txt"), []byte("behind"), 0o644); err != nil {
		t.Fatalf("write behind file: %v", err)
	}
	runGitCommandInDir(t, peerDir, "add", "behind.txt")
	runGitCommandInDir(t, peerDir, "commit", "-m", "behind commit")
	runGitCommandInDir(t, peerDir, "push", "origin", branchName)

	runGitCommandInDir(t, cloneDir, "fetch", "origin")

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	infos, err := repo.listLocalBranchTrackingInfo()
	if err != nil {
		t.Fatalf("listLocalBranchTrackingInfo() error = %v", err)
	}

	var target branchTrackingInfo
	found := false
	for _, info := range infos {
		if info.Name == branchName {
			target = info
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("branch %q not found in tracking info: %+v", branchName, infos)
	}

	if target.Upstream != "origin/"+branchName {
		t.Fatalf("Upstream = %q, want %q", target.Upstream, "origin/"+branchName)
	}
	if !strings.Contains(target.UpstreamTrack, "ahead") || !strings.Contains(target.UpstreamTrack, "behind") {
		t.Fatalf("UpstreamTrack = %q, want both ahead/behind status", target.UpstreamTrack)
	}
	if !strings.Contains(target.UpstreamTrack, ",") {
		t.Fatalf("UpstreamTrack = %q, want combined status with comma", target.UpstreamTrack)
	}
}

func TestListRemoteBranchNamesParsesSlashBranchName(t *testing.T) {
	testutil.SkipIfNoGit(t)

	_, cloneDir := createBareAndClone(t)
	branchName := "feature/team/task-123"
	runGitCommandInDir(t, cloneDir, "push", "origin", "HEAD:refs/heads/"+branchName)
	runGitCommandInDir(t, cloneDir, "fetch", "origin")

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	names, err := repo.listRemoteBranchNames()
	if err != nil {
		t.Fatalf("listRemoteBranchNames() error = %v", err)
	}
	if _, ok := names[branchName]; !ok {
		t.Fatalf("remote branch %q not found in map: %v", branchName, names)
	}
}

func TestListRemoteBranchNamesSupportsRemoteNameWithSlash(t *testing.T) {
	testutil.SkipIfNoGit(t)

	bareDir, cloneDir := createBareAndClone(t)
	remoteName := "team/origin"
	branchName := "feature/remote-with-slash"

	runGitCommandInDir(t, cloneDir, "remote", "add", remoteName, bareDir)
	runGitCommandInDir(t, cloneDir, "push", remoteName, "HEAD:refs/heads/"+branchName)
	runGitCommandInDir(t, cloneDir, "fetch", remoteName)

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	names, err := repo.listRemoteBranchNames()
	if err != nil {
		t.Fatalf("listRemoteBranchNames() error = %v", err)
	}
	if _, ok := names[branchName]; !ok {
		t.Fatalf("remote branch %q not found in map: %v", branchName, names)
	}
}

func TestCleanupLocalBranchIfOrphaned(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name            string
		setup           func(t *testing.T) (*Repository, string)
		wantDeleted     bool
		wantErr         bool
		wantErrContains string
		verify          func(t *testing.T, repo *Repository, branchName string)
	}{
		{
			name: "invalid branch name returns validation error",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "../invalid"
			},
			wantDeleted: false,
			wantErr:     true,
		},
		{
			name: "missing branch returns false without error",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/missing"
			},
			wantDeleted: false,
			wantErr:     false,
		},
		{
			name: "branch with live upstream is preserved",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				defaultBranch := runGitCommandInDir(t, cloneDir, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, cloneDir, "checkout", "-b", "feature/live")
				runGitCommandInDir(t, cloneDir, "push", "-u", "origin", "feature/live")
				runGitCommandInDir(t, cloneDir, "checkout", defaultBranch)

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/live"
			},
			wantDeleted: false,
			wantErr:     false,
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, true)
			},
		},
		{
			name: "branch matching remote name without upstream is preserved",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				runGitCommandInDir(t, cloneDir, "push", "origin", "HEAD:refs/heads/feature/remote-only")
				runGitCommandInDir(t, cloneDir, "branch", "--no-track", "feature/remote-only")

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/remote-only"
			},
			wantDeleted: false,
			wantErr:     false,
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, true)
			},
		},
		{
			name: "orphan branch is deleted",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				runGitCommandInDir(t, repoPath, "branch", "feature/orphan")

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/orphan"
			},
			wantDeleted: true,
			wantErr:     false,
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, false)
			},
		},
		{
			name: "detached HEAD still deletes orphan branch",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				runGitCommandInDir(t, repoPath, "branch", "feature/orphan-detached")
				runGitCommandInDir(t, repoPath, "checkout", "--detach")

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/orphan-detached"
			},
			wantDeleted: true,
			wantErr:     false,
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, false)
			},
		},
		{
			name: "checked out orphan branch is preserved without delete attempt",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				currentBranch := runGitCommandInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, currentBranch
			},
			wantDeleted: false,
			wantErr:     false,
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, true)
			},
		},
		{
			name: "branch with remote present but upstream gone is deleted",
			setup: func(t *testing.T) (*Repository, string) {
				_, cloneDir := createBareAndClone(t)
				defaultBranch := runGitCommandInDir(t, cloneDir, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, cloneDir, "checkout", "-b", "feature/remote-gone")
				runGitCommandInDir(t, cloneDir, "push", "-u", "origin", "feature/remote-gone")
				runGitCommandInDir(t, cloneDir, "checkout", defaultBranch)
				// No fetch --prune is needed here because deleting via this same clone
				// updates its remote-tracking refs immediately.
				runGitCommandInDir(t, cloneDir, "push", "origin", "--delete", "feature/remote-gone")

				repo, err := Open(cloneDir)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/remote-gone"
			},
			wantDeleted: true,
			wantErr:     false,
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, false)
			},
		},
		{
			name: "CurrentBranch error returns descriptive error",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				runGitCommandInDir(t, repoPath, "branch", "feature/cb-error")

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}

				// Point HEAD at an empty branch name ("ref: refs/heads/\n").
				// for-each-ref still enumerates refs without resolving HEAD.
				// rev-parse --abbrev-ref HEAD fails because the ref is unresolvable.
				headPath := filepath.Join(repoPath, ".git", "HEAD")
				if err := os.WriteFile(headPath, []byte("ref: refs/heads/\n"), 0o644); err != nil {
					t.Fatalf("failed to corrupt HEAD: %v", err)
				}

				return repo, "feature/cb-error"
			},
			wantDeleted:     false,
			wantErr:         true,
			wantErrContains: "failed to determine current branch",
		},
		{
			name: "delete failure for unmerged branch is returned",
			setup: func(t *testing.T) (*Repository, string) {
				repoPath := testutil.CreateTempGitRepo(t)
				defaultBranch := runGitCommandInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
				runGitCommandInDir(t, repoPath, "checkout", "-b", "feature/unmerged")
				if err := os.WriteFile(filepath.Join(repoPath, "unmerged.txt"), []byte("unmerged"), 0o644); err != nil {
					t.Fatalf("write unmerged commit file: %v", err)
				}
				runGitCommandInDir(t, repoPath, "add", "unmerged.txt")
				runGitCommandInDir(t, repoPath, "commit", "-m", "unmerged commit")
				runGitCommandInDir(t, repoPath, "checkout", defaultBranch)

				repo, err := Open(repoPath)
				if err != nil {
					t.Fatalf("Open() error = %v", err)
				}
				return repo, "feature/unmerged"
			},
			wantDeleted:     false,
			wantErr:         true,
			wantErrContains: "may have unmerged commits",
			verify: func(t *testing.T, repo *Repository, branchName string) {
				assertBranchPresence(t, repo, branchName, true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, branchName := tt.setup(t)

			deleted, err := repo.CleanupLocalBranchIfOrphaned(branchName)
			if tt.wantErr {
				if err == nil {
					t.Fatal("CleanupLocalBranchIfOrphaned() expected error")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErrContains)
				}
			} else if err != nil {
				t.Fatalf("CleanupLocalBranchIfOrphaned() unexpected error = %v", err)
			}
			if deleted != tt.wantDeleted {
				t.Fatalf("CleanupLocalBranchIfOrphaned() deleted = %v, want %v", deleted, tt.wantDeleted)
			}
			if tt.verify != nil {
				tt.verify(t, repo, branchName)
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
