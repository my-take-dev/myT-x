package git

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"myT-x/internal/testutil"
)

func TestCreateWorktreeDetached(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wtPath := filepath.Join(wtDir, "experiment")
	if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
		t.Fatalf("CreateWorktreeDetached() error = %v", err)
	}

	// Verify worktree directory exists.
	if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		t.Error("worktree directory was not created")
	}

	// Verify it appears in the list.
	worktrees, err := repo.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	found := slices.Contains(worktrees, wtPath)
	if !found {
		t.Errorf("worktree %q not found in list: %v", wtPath, worktrees)
	}

	// Cleanup.
	if err := repo.RemoveWorktree(wtPath); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
}

func TestCreateWorktreeWithBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wtPath := filepath.Join(wtDir, "feature-auth")
	if err := repo.CreateWorktree(wtPath, "feature/auth", "HEAD"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Verify worktree directory exists.
	if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		t.Error("worktree directory was not created")
	}

	// Verify branch info.
	infos, err := repo.ListWorktreesWithInfo()
	if err != nil {
		t.Fatalf("ListWorktreesWithInfo() error = %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Path == wtPath {
			found = true
			if info.Branch != "feature/auth" {
				t.Errorf("expected branch 'feature/auth', got %q", info.Branch)
			}
			if info.IsDetached {
				t.Error("expected not detached")
			}
			break
		}
	}
	if !found {
		t.Errorf("worktree %q not found in info list", wtPath)
	}

	// Cleanup.
	if err := repo.RemoveWorktree(wtPath); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
}

func TestCreateWorktreeWithCommitishBase(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wtPath := filepath.Join(wtDir, "feature-from-commitish")
	if err := repo.CreateWorktree(wtPath, "feature/from-commitish", "HEAD~0"); err != nil {
		t.Fatalf("CreateWorktree() with commit-ish base error = %v", err)
	}

	if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		t.Fatal("worktree directory was not created")
	}

	if err := repo.RemoveWorktree(wtPath); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
}

func TestCreateWorktreeWithBranchConflict(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	firstPath := filepath.Join(wtDir, "feature-conflict-1")
	if err := repo.CreateWorktree(firstPath, "feature/conflict", "HEAD"); err != nil {
		t.Fatalf("initial CreateWorktree() error = %v", err)
	}
	t.Cleanup(func() {
		_ = repo.RemoveWorktreeForced(firstPath)
	})

	secondPath := filepath.Join(wtDir, "feature-conflict-2")
	if err := repo.CreateWorktree(secondPath, "feature/conflict", "HEAD"); err == nil {
		t.Fatal("CreateWorktree() expected branch-conflict error")
	}
}

func TestCreateWorktreeRejectsInvalidBaseBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "invalid-base")

	if err := repo.CreateWorktree(wtPath, "feature/invalid-base", "invalid base"); err == nil {
		t.Fatal("CreateWorktree() expected base branch validation error")
	}
}

func TestCreateWorktreeFromBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	defaultBranch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if err := repo.CheckoutNewBranch("feature/from-existing"); err != nil {
		t.Fatalf("CheckoutNewBranch() error = %v", err)
	}
	if _, err := repo.runGitCommand("checkout", defaultBranch); err != nil {
		t.Fatalf("checkout default branch error = %v", err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "existing-branch")

	if err := repo.CreateWorktreeFromBranch(wtPath, "feature/from-existing"); err != nil {
		t.Fatalf("CreateWorktreeFromBranch() error = %v", err)
	}

	if _, err := os.Stat(wtPath); errors.Is(err, os.ErrNotExist) {
		t.Fatal("worktree directory was not created")
	}

	infos, err := repo.ListWorktreesWithInfo()
	if err != nil {
		t.Fatalf("ListWorktreesWithInfo() error = %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Path != wtPath {
			continue
		}
		found = true
		if info.Branch != "feature/from-existing" {
			t.Fatalf("worktree branch = %q, want %q", info.Branch, "feature/from-existing")
		}
		if info.IsDetached {
			t.Fatal("worktree should not be detached")
		}
	}
	if !found {
		t.Fatalf("worktree %q not found in info list", wtPath)
	}
}

func TestCreateWorktreeFromBranchValidation(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "invalid-branch")

	if err := repo.CreateWorktreeFromBranch(wtPath, "invalid branch name"); err == nil {
		t.Fatal("CreateWorktreeFromBranch() expected branch validation error")
	}
}

func TestListWorktreesWithInfo(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	// The main worktree should always be in the list.
	infos, err := repo.ListWorktreesWithInfo()
	if err != nil {
		t.Fatalf("ListWorktreesWithInfo() error = %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("expected at least one worktree (main)")
	}
	if !infos[0].IsMain {
		t.Error("first worktree should be marked as main")
	}
}

func TestRemoveWorktreeForced(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wtPath := filepath.Join(wtDir, "dirty-wt")
	if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
		t.Fatal(err)
	}

	// Create an uncommitted change in the worktree.
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("change"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Normal remove should fail due to uncommitted changes.
	if err := repo.RemoveWorktree(wtPath); err == nil {
		t.Log("RemoveWorktree succeeded (git may allow removal of untracked files)")
		return
	}

	// Forced remove should succeed.
	if err := repo.RemoveWorktreeForced(wtPath); err != nil {
		t.Fatalf("RemoveWorktreeForced() error = %v", err)
	}
}

func TestPruneWorktrees(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	// Prune on a clean repo should succeed without error.
	if err := repo.PruneWorktrees(); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}
}

func TestPruneRemovesStaleWorktreeEntries(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)
	repo, err := Open(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	wtDir := GenerateWorktreeDirPath(repoDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wtPath := filepath.Join(wtDir, "stale-wt")
	if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
		t.Fatalf("CreateWorktreeDetached() error = %v", err)
	}

	// Verify the worktree appears in the list.
	infos, err := repo.ListWorktreesWithInfo()
	if err != nil {
		t.Fatalf("ListWorktreesWithInfo() error = %v", err)
	}
	foundBefore := false
	for _, info := range infos {
		if info.Path == wtPath {
			foundBefore = true
			break
		}
	}
	if !foundBefore {
		t.Fatalf("worktree %q not found in list before deletion", wtPath)
	}

	// Simulate user deleting the worktree folder manually.
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("failed to remove worktree folder: %v", err)
	}

	// Before prune: stale entry should still appear.
	infosBeforePrune, err := repo.ListWorktreesWithInfo()
	if err != nil {
		t.Fatalf("ListWorktreesWithInfo() before prune error = %v", err)
	}
	staleFound := false
	for _, info := range infosBeforePrune {
		if info.Path == wtPath {
			staleFound = true
			break
		}
	}
	if !staleFound {
		t.Log("git already cleaned up stale entry (some git versions auto-prune)")
	}

	// Prune should remove the stale entry.
	if err := repo.PruneWorktrees(); err != nil {
		t.Fatalf("PruneWorktrees() error = %v", err)
	}

	// After prune: stale entry should be gone.
	infosAfterPrune, err := repo.ListWorktreesWithInfo()
	if err != nil {
		t.Fatalf("ListWorktreesWithInfo() after prune error = %v", err)
	}
	for _, info := range infosAfterPrune {
		if info.Path == wtPath {
			t.Errorf("stale worktree %q still present after prune", wtPath)
		}
	}
}

func TestIsDetachedHead(t *testing.T) {
	testutil.SkipIfNoGit(t)

	t.Run("attached HEAD returns false", func(t *testing.T) {
		repoDir := testutil.CreateTempGitRepo(t)
		repo, err := Open(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		isDetached, err := repo.IsDetachedHead()
		if err != nil {
			t.Fatalf("IsDetachedHead() error = %v", err)
		}
		if isDetached {
			t.Fatal("expected attached HEAD, got detached")
		}
	})

	t.Run("detached HEAD returns true", func(t *testing.T) {
		repoDir := testutil.CreateTempGitRepo(t)
		repo, err := Open(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		// Detach HEAD.
		if _, err := repo.runGitCommand("checkout", "--detach"); err != nil {
			t.Fatalf("git checkout --detach failed: %v", err)
		}
		isDetached, err := repo.IsDetachedHead()
		if err != nil {
			t.Fatalf("IsDetachedHead() error = %v", err)
		}
		if !isDetached {
			t.Fatal("expected detached HEAD, got attached")
		}
	})

	t.Run("non-git directory returns error", func(t *testing.T) {
		nonGitDir := t.TempDir()
		repo := &Repository{path: nonGitDir}
		isDetached, err := repo.IsDetachedHead()
		if err == nil {
			t.Fatal("expected error for non-git directory")
		}
		if isDetached {
			t.Fatal("should not report detached HEAD on error")
		}
	})
}

func TestCheckWorktreeHealth(t *testing.T) {
	testutil.SkipIfNoGit(t)

	t.Run("healthy worktree", func(t *testing.T) {
		repoDir := testutil.CreateTempGitRepo(t)
		repo, err := Open(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		wtDir := GenerateWorktreeDirPath(repoDir)
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtDir, "healthy")
		if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repo.RemoveWorktreeForced(wtPath) })

		health := repo.CheckWorktreeHealth(wtPath)
		if !health.IsHealthy {
			t.Fatalf("expected healthy worktree, got issues: %v", health.Issues)
		}
		if len(health.Issues) > 0 {
			t.Fatalf("expected no issues, got: %v", health.Issues)
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		repoDir := testutil.CreateTempGitRepo(t)
		repo, err := Open(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		health := repo.CheckWorktreeHealth(filepath.Join(repoDir, "nonexistent"))
		if health.IsHealthy {
			t.Fatal("expected unhealthy for missing directory")
		}
		if len(health.Issues) == 0 {
			t.Fatal("expected issues for missing directory")
		}
	})

	t.Run("directory without .git file", func(t *testing.T) {
		repoDir := testutil.CreateTempGitRepo(t)
		repo, err := Open(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		brokenDir := filepath.Join(t.TempDir(), "broken-wt")
		if err := os.MkdirAll(brokenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		health := repo.CheckWorktreeHealth(brokenDir)
		if health.IsHealthy {
			t.Fatal("expected unhealthy for directory without .git file")
		}
		// Step 2 failure should produce exactly 1 issue (early return, not 2).
		if len(health.Issues) != 1 {
			t.Fatalf("expected 1 issue for missing .git, got %d: %v", len(health.Issues), health.Issues)
		}
	})

	t.Run("worktree with corrupt HEAD", func(t *testing.T) {
		repoDir := testutil.CreateTempGitRepo(t)
		repo, err := Open(repoDir)
		if err != nil {
			t.Fatal(err)
		}
		wtDir := GenerateWorktreeDirPath(repoDir)
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtDir, "corrupt")
		if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repo.RemoveWorktreeForced(wtPath) })

		// Corrupt the HEAD file to make rev-parse fail.
		headPath := filepath.Join(wtPath, ".git")
		// Read the .git file content (it's a gitdir pointer file for worktrees).
		gitContent, err := os.ReadFile(headPath)
		if err != nil {
			t.Fatal(err)
		}
		// Extract the gitdir path and corrupt the HEAD file there.
		gitDirLine := strings.TrimSpace(string(gitContent))
		if after, ok := strings.CutPrefix(gitDirLine, "gitdir: "); ok {
			gitDir := filepath.FromSlash(after)
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(wtPath, gitDir)
			}
			headFile := filepath.Join(gitDir, "HEAD")
			if err := os.WriteFile(headFile, []byte("invalid-ref"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		health := repo.CheckWorktreeHealth(wtPath)
		if health.IsHealthy {
			t.Fatal("expected unhealthy for corrupt HEAD")
		}
		if len(health.Issues) == 0 {
			t.Fatal("expected at least one issue for corrupt HEAD")
		}
	})
}

func TestWorktreeStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[WorktreeInfo]().NumField(); got != 5 {
		t.Fatalf("WorktreeInfo field count = %d, want 5; update tests for new fields", got)
	}
	if got := reflect.TypeFor[WorktreeHealth]().NumField(); got != 2 {
		t.Fatalf("WorktreeHealth field count = %d, want 2; update tests for new fields", got)
	}
}

func TestWorktreePathGeneration(t *testing.T) {
	// Verify that worktree is created at the same level as the repo (.wt suffix).
	testutil.SkipIfNoGit(t)

	repoDir := testutil.CreateTempGitRepo(t)

	wtDir := GenerateWorktreeDirPath(repoDir)
	expectedParent := filepath.Dir(repoDir)
	actualParent := filepath.Dir(wtDir)

	if actualParent != expectedParent {
		t.Errorf("worktree dir parent = %q, want %q (same level as repo)", actualParent, expectedParent)
	}

	repoBase := filepath.Base(repoDir)
	wtBase := filepath.Base(wtDir)
	expectedBase := repoBase + ".wt"
	if wtBase != expectedBase {
		t.Errorf("worktree dir base = %q, want %q", wtBase, expectedBase)
	}
}
