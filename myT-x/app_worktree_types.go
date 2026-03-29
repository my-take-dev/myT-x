package main

import (
	gitpkg "myT-x/internal/git"
	"myT-x/internal/worktree"
)

// Type aliases for Wails binding compatibility.
// Wails generates bindings from App method signatures in the main package.
// These aliases re-export internal worktree types so that Wails can
// discover them without exposing the internal package directly.
type WorktreeSessionOptions = worktree.WorktreeSessionOptions
type WorktreeStatus = worktree.WorktreeStatus
type OrphanedWorktree = worktree.OrphanedWorktree
type WorktreeHealth = gitpkg.WorktreeHealth
