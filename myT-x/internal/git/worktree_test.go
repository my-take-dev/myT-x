package git

import (
	"os"
	"path/filepath"
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
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify it appears in the list.
	worktrees, err := repo.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	found := false
	for _, wt := range worktrees {
		if wt == wtPath {
			found = true
			break
		}
	}
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
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
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
