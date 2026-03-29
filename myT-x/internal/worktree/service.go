package worktree

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/procutil"
	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// Deps — external dependencies injected at construction time
// ---------------------------------------------------------------------------

// Deps holds external dependencies injected at construction time.
// All function fields except Emitter and IsShuttingDown must be non-nil.
// NewService panics if any required function field is nil.
type Deps struct {
	// Optional: defaults to a no-op emitter if nil.
	Emitter apptypes.RuntimeEventEmitter

	// Optional: defaults to func() bool { return false } if nil.
	IsShuttingDown func() bool

	// RequireSessions returns the session manager for metadata operations.
	RequireSessions func() (*tmux.SessionManager, error)

	// RequireSessionsAndRouter verifies that both session manager and command
	// router are available. Returns sessions for subsequent metadata operations.
	// The router is checked but not returned; session creation uses CreateSession.
	RequireSessionsAndRouter func() (*tmux.SessionManager, error)

	// GetConfigSnapshot returns a deep copy of the current configuration.
	GetConfigSnapshot func() config.Config

	// RuntimeContext returns the application runtime context.
	RuntimeContext func() context.Context

	// FindAvailableSessionName deduplicates session names by appending suffixes.
	FindAvailableSessionName func(name string) string

	// CreateSession creates a tmux session in the given directory.
	// The router is managed internally by the implementation.
	//
	// NOTE: UseSessionPaneScope is intentionally excluded from the parameter
	// list. createSessionForDirectory (the underlying implementation) only needs
	// the three flags that affect initial pane env setup. UseSessionPaneScope is
	// applied separately via ApplySessionEnvFlags after session creation.
	CreateSession func(sessionDir, sessionName string, enableAgentTeam, useClaudeEnv, usePaneEnv bool) (createdName string, err error)

	// ApplySessionEnvFlags sets session-level env flags after creation.
	ApplySessionEnvFlags func(sm *tmux.SessionManager, sessionName string, useClaudeEnv, usePaneEnv, useSessionPaneScope bool)

	// ActivateCreatedSession sets the session as active and returns its snapshot.
	ActivateCreatedSession func(createdName string) (tmux.SessionSnapshot, error)

	// RollbackCreatedSession destroys a session on creation failure.
	RollbackCreatedSession func(sessionName string) error

	// StoreRootPath saves the root directory for conflict detection.
	StoreRootPath func(sessionName, rootPath string) error

	// RequestSnapshot triggers a snapshot refresh to sync the frontend.
	RequestSnapshot func(force bool)

	// FindSessionByWorktreePath checks for worktree path conflicts.
	FindSessionByWorktreePath func(wtPath string) string

	// EmitWorktreeCleanupFailure notifies the frontend of cleanup failure.
	EmitWorktreeCleanupFailure func(sessionName, wtPath string, err error)

	// CleanupOrphanedLocalBranch removes orphaned branches after worktree cleanup.
	CleanupOrphanedLocalBranch func(sessionName string, repo *gitpkg.Repository, branchName string)

	// SetupWGAdd increments the setup WaitGroup counter for async scripts.
	SetupWGAdd func(delta int)

	// SetupWGDone decrements the setup WaitGroup counter.
	SetupWGDone func()

	// RecoverBackgroundPanic handles panics in background goroutines.
	RecoverBackgroundPanic func(worker string, recovered any) bool

	// --- IO operations (optional, defaults to stdlib) ---

	// CurrentBranch resolves the current branch of a git repository.
	// Defaults to repo.CurrentBranch().
	CurrentBranch func(repo *gitpkg.Repository) (string, error)

	// ExecuteSetupCommand runs a setup script in a directory.
	// Defaults to exec.CommandContext with HideWindow.
	ExecuteSetupCommand func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error)

	// Copy holds file I/O dependencies used exclusively by worktree copy
	// operations (CopyConfigFilesToWorktree, CopyConfigDirsToWorktree).
	// All fields default to stdlib equivalents if zero-valued.
	Copy CopyDeps
}

// CopyDeps holds file I/O dependencies used exclusively by worktree
// copy operations (CopyConfigFilesToWorktree, CopyConfigDirsToWorktree).
type CopyDeps struct {
	// WalkDir traverses a directory tree.
	// Defaults to filepath.WalkDir.
	WalkDir func(root string, fn fs.WalkDirFunc) error

	// StreamCopy copies data between streams.
	// Defaults to io.Copy.
	StreamCopy func(dst io.Writer, src io.Reader) (int64, error)

	// SyncFile flushes file data to stable storage.
	// Defaults to file.Sync().
	SyncFile func(file *os.File) error

	// StatFileInfo returns file metadata.
	// Defaults to os.Stat.
	StatFileInfo func(name string) (os.FileInfo, error)

	// RemoveFile deletes a file.
	// Defaults to os.Remove.
	RemoveFile func(name string) error

	// MaxCopyDirsFileCount is the maximum file count for copy_dirs operations.
	// Defaults to 10,000.
	MaxCopyDirsFileCount int

	// MaxCopyDirsTotalBytes is the maximum total size for copy_dirs operations.
	// Defaults to 500 MiB.
	MaxCopyDirsTotalBytes int64
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service encapsulates worktree lifecycle management.
// Stateless: all session state lives in SessionManager (internal lock).
// No App-level or Service-level mutex is needed.
type Service struct {
	deps Deps
}

// NewService creates a worktree service with the given dependencies.
// Panics if any required function field in deps is nil, reporting which fields are missing.
func NewService(deps Deps) *Service {
	var missing []string
	if deps.RequireSessions == nil {
		missing = append(missing, "RequireSessions")
	}
	if deps.RequireSessionsAndRouter == nil {
		missing = append(missing, "RequireSessionsAndRouter")
	}
	if deps.GetConfigSnapshot == nil {
		missing = append(missing, "GetConfigSnapshot")
	}
	if deps.RuntimeContext == nil {
		missing = append(missing, "RuntimeContext")
	}
	if deps.FindAvailableSessionName == nil {
		missing = append(missing, "FindAvailableSessionName")
	}
	if deps.CreateSession == nil {
		missing = append(missing, "CreateSession")
	}
	if deps.ApplySessionEnvFlags == nil {
		missing = append(missing, "ApplySessionEnvFlags")
	}
	if deps.ActivateCreatedSession == nil {
		missing = append(missing, "ActivateCreatedSession")
	}
	if deps.RollbackCreatedSession == nil {
		missing = append(missing, "RollbackCreatedSession")
	}
	if deps.StoreRootPath == nil {
		missing = append(missing, "StoreRootPath")
	}
	if deps.RequestSnapshot == nil {
		missing = append(missing, "RequestSnapshot")
	}
	if deps.FindSessionByWorktreePath == nil {
		missing = append(missing, "FindSessionByWorktreePath")
	}
	if deps.EmitWorktreeCleanupFailure == nil {
		missing = append(missing, "EmitWorktreeCleanupFailure")
	}
	if deps.CleanupOrphanedLocalBranch == nil {
		missing = append(missing, "CleanupOrphanedLocalBranch")
	}
	if deps.SetupWGAdd == nil {
		missing = append(missing, "SetupWGAdd")
	}
	if deps.SetupWGDone == nil {
		missing = append(missing, "SetupWGDone")
	}
	if deps.RecoverBackgroundPanic == nil {
		missing = append(missing, "RecoverBackgroundPanic")
	}
	if len(missing) > 0 {
		panic("worktree.NewService: nil deps: " + strings.Join(missing, ", "))
	}
	if deps.IsShuttingDown == nil {
		deps.IsShuttingDown = func() bool { return false }
	}
	if deps.Emitter == nil {
		slog.Debug("[DEBUG-WORKTREE] NewService: Emitter is nil, using NoopEmitter")
		deps.Emitter = apptypes.NoopEmitter{}
	}
	if deps.CurrentBranch == nil {
		deps.CurrentBranch = func(repo *gitpkg.Repository) (string, error) {
			return repo.CurrentBranch()
		}
	}
	if deps.ExecuteSetupCommand == nil {
		deps.ExecuteSetupCommand = func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, shell, shellFlag, script)
			cmd.Dir = dir
			procutil.HideWindow(cmd)
			return cmd.CombinedOutput()
		}
	}
	if deps.Copy.WalkDir == nil {
		deps.Copy.WalkDir = filepath.WalkDir
	}
	if deps.Copy.StreamCopy == nil {
		deps.Copy.StreamCopy = io.Copy
	}
	if deps.Copy.SyncFile == nil {
		deps.Copy.SyncFile = func(file *os.File) error { return file.Sync() }
	}
	if deps.Copy.StatFileInfo == nil {
		deps.Copy.StatFileInfo = os.Stat
	}
	if deps.Copy.RemoveFile == nil {
		deps.Copy.RemoveFile = os.Remove
	}
	if deps.Copy.MaxCopyDirsFileCount == 0 {
		deps.Copy.MaxCopyDirsFileCount = 10_000
	}
	if deps.Copy.MaxCopyDirsTotalBytes == 0 {
		deps.Copy.MaxCopyDirsTotalBytes = 500 * 1024 * 1024
	}
	return &Service{deps: deps}
}
