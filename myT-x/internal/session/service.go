package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// Deps — external dependencies injected at construction time
// ---------------------------------------------------------------------------

// Deps holds external dependencies injected at construction time.
// All function fields except Emitter and IsShuttingDown must be non-nil.
// NewService panics if any required function field is nil.
type Deps struct {
	// --- Optional (defaults applied by NewService) ---

	// Emitter sends events directly to the frontend via Wails runtime.
	// Defaults to NoopEmitter if nil.
	Emitter apptypes.RuntimeEventEmitter

	// IsShuttingDown reports whether the application is shutting down.
	// Lifecycle-mutating methods (CreateSession, RenameSession) check this
	// and return early to prevent state changes against a partially torn-down
	// runtime. Defaults to func() bool { return false } if nil.
	IsShuttingDown func() bool

	// --- tmux infrastructure (required) ---

	// RequireSessions returns the session manager for metadata operations.
	RequireSessions func() (*tmux.SessionManager, error)

	// RequireRouter returns the command router for IPC operations.
	RequireRouter func() (*tmux.CommandRouter, error)

	// --- Application state (required) ---

	// GetConfigSnapshot returns a deep copy of the current configuration.
	GetConfigSnapshot func() config.Config

	// RuntimeContext returns the application runtime context.
	// Returns nil during shutdown, which is used as a guard in EmitWithContext.
	RuntimeContext func() context.Context

	// --- Event notification (required) ---

	// RequestSnapshot triggers a snapshot refresh to sync the frontend.
	RequestSnapshot func(force bool)

	// EmitBackendEvent emits a backend event with snapshot policy awareness.
	// Unlike Emitter.Emit (which goes directly to the frontend via Wails),
	// EmitBackendEvent routes through the App layer's snapshot event policies,
	// potentially triggering additional snapshot refreshes based on the event name.
	// Use EmitBackendEvent for events that should trigger UI state reconciliation
	// (e.g. "tmux:session-renamed" → snapshot policy auto-triggers RequestSnapshot).
	// Use Emitter.Emit for lightweight frontend notifications without snapshot side-effects.
	EmitBackendEvent func(name string, payload any)

	// --- Cleanup (required) ---

	// McpCleanupSession releases MCP instance state for a destroyed or renamed
	// session. The closure must handle nil-guard for the MCP manager internally.
	McpCleanupSession func(sessionName string)

	// CleanupStaleSnapshotState removes stale pane output buffers and state
	// after session destruction. Called with the current set of active pane IDs.
	CleanupStaleSnapshotState func(activePaneIDs map[string]struct{})

	// --- IO operations (optional, defaults to stdlib) ---

	// ExecuteRouterRequest dispatches a request to the command router.
	// Defaults to router.Execute(req).
	ExecuteRouterRequest func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service encapsulates session lifecycle management: create, rename, kill,
// active session tracking, session lookup, and worktree cleanup.
//
// Thread-safety: activeMu guards activeSess. All other session state lives in
// SessionManager (internal lock). No additional Service-level lock is needed.
type Service struct {
	deps       Deps
	activeMu   sync.RWMutex
	activeSess string
}

// NewService creates a session service with the given dependencies.
// Panics if any required function field in deps is nil, reporting which fields are missing.
func NewService(deps Deps) *Service {
	var missing []string
	if deps.RequireSessions == nil {
		missing = append(missing, "RequireSessions")
	}
	if deps.RequireRouter == nil {
		missing = append(missing, "RequireRouter")
	}
	if deps.GetConfigSnapshot == nil {
		missing = append(missing, "GetConfigSnapshot")
	}
	if deps.RuntimeContext == nil {
		missing = append(missing, "RuntimeContext")
	}
	if deps.RequestSnapshot == nil {
		missing = append(missing, "RequestSnapshot")
	}
	if deps.EmitBackendEvent == nil {
		missing = append(missing, "EmitBackendEvent")
	}
	if deps.McpCleanupSession == nil {
		missing = append(missing, "McpCleanupSession")
	}
	if deps.CleanupStaleSnapshotState == nil {
		missing = append(missing, "CleanupStaleSnapshotState")
	}
	if len(missing) > 0 {
		panic("session.NewService: nil deps: " + strings.Join(missing, ", "))
	}
	if deps.Emitter == nil {
		slog.Debug("[DEBUG-SESSION] NewService: Emitter is nil, using NoopEmitter")
		deps.Emitter = apptypes.NoopEmitter{}
	}
	if deps.IsShuttingDown == nil {
		deps.IsShuttingDown = func() bool { return false }
	}
	if deps.ExecuteRouterRequest == nil {
		deps.ExecuteRouterRequest = func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			return router.Execute(req)
		}
	}
	return &Service{deps: deps}
}

// requireSessionsAndRouter validates both session manager and router
// availability. Consolidates the repeated two-step guard pattern.
func (s *Service) requireSessionsAndRouter() (*tmux.SessionManager, *tmux.CommandRouter, error) {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return nil, nil, err
	}
	router, err := s.deps.RequireRouter()
	if err != nil {
		return nil, nil, err
	}
	return sessions, router, nil
}

// ===========================================================================
// Active session management
// ===========================================================================

// SetActiveSessionName is a low-level API that normalizes and stores the active
// session name without emitting events. Prefer SetActive for external callers.
//
// Internal use only: called by SetActive, KillSession (to clear the name),
// and QuickStartSession (conflict path with policy-aware emission).
// Direct use from outside the session package is acceptable only in tests or
// when the caller manages event emission separately.
func (s *Service) SetActiveSessionName(sessionName string) string {
	normalized := strings.TrimSpace(sessionName)
	s.activeMu.Lock()
	s.activeSess = normalized
	s.activeMu.Unlock()
	return normalized
}

// GetActiveSessionName returns the currently active session name.
func (s *Service) GetActiveSessionName() string {
	s.activeMu.RLock()
	name := s.activeSess
	s.activeMu.RUnlock()
	return name
}

// SetActive sets the active session and emits a frontend event via Emitter.
// This is the high-level API used by the Wails-bound SetActiveSession wrapper.
// During shutdown, the event emission is skipped to avoid reaching a partially
// torn-down runtime; the name is still stored for internal consistency.
func (s *Service) SetActive(sessionName string) {
	name := s.SetActiveSessionName(sessionName)
	if s.deps.IsShuttingDown() {
		slog.Debug("[DEBUG-SESSION] SetActive: event emission skipped during shutdown",
			"session", name)
		return
	}
	s.deps.Emitter.Emit("tmux:active-session", map[string]string{"name": name})
}

// ===========================================================================
// Session lifecycle — create, rename, kill, quick-start
// ===========================================================================

// QuickStartSession creates a session using the configured default directory
// (config.DefaultSessionDir) or launchDir as a fallback.
// If the directory is already in use by an existing session, that session is
// activated and returned instead of creating a new one.
func (s *Service) QuickStartSession(launchDir string) (tmux.SessionSnapshot, error) {
	cfg := s.deps.GetConfigSnapshot()
	dir := strings.TrimSpace(cfg.DefaultSessionDir)
	if dir == "" {
		dir = launchDir
	}
	if dir == "" {
		return tmux.SessionSnapshot{}, errors.New("no directory available for quick start session")
	}

	// [C2] Environment variables and ~ are expanded by config.validateDefaultSessionDir
	// at load/save time. This guard handles direct API calls with unexpanded paths.
	if strings.HasPrefix(dir, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[1:])
		}
	}

	// Resolve symlinks up front to avoid creating duplicate sessions for
	// path aliases that point to the same directory.
	if resolvedDir, evalErr := filepath.EvalSymlinks(dir); evalErr == nil {
		dir = resolvedDir
	} else if !errors.Is(evalErr, os.ErrNotExist) {
		slog.Debug("[DEBUG-SESSION] quick start path canonicalization skipped",
			"path", dir, "error", evalErr)
	}

	// [I1] os.MkdirAll is idempotent; calling it on an existing directory returns nil.
	// We call it unconditionally to avoid the TOCTOU race of Stat → MkdirAll.
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("quick start directory create failed: %w", mkErr)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return tmux.SessionSnapshot{}, fmt.Errorf("quick start directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return tmux.SessionSnapshot{}, fmt.Errorf("quick start path is not a directory: %s", dir)
	}

	// If directory is already used by an existing session, activate it.
	// NOTE: This check is intentionally best-effort. If the session disappears
	// between FindSessionByRootPath and the snapshot loop (TOCTOU window), we
	// safely fall through to CreateSession.
	if conflict := s.FindSessionByRootPath(dir); conflict != "" {
		// NOTE: FindSessionByRootPath internally calls RequireSessions().Snapshot()
		// but only returns the session name. We need a fresh SessionSnapshot to
		// return to the caller, hence this second RequireSessions() call.
		// The Snapshot() call itself is a lightweight in-memory copy.
		sessions, sErr := s.deps.RequireSessions()
		if sErr == nil {
			for _, snap := range sessions.Snapshot() {
				if snap.Name == conflict {
					s.SetActiveSessionName(conflict)
					s.deps.RequestSnapshot(true)
					s.deps.EmitBackendEvent("tmux:active-session", map[string]any{"name": conflict})
					return snap, nil
				}
			}
		} else {
			slog.Debug("[DEBUG-SESSION] quick start conflict snapshot unavailable; creating new session",
				"session", conflict, "path", dir, "error", sErr)
		}
		// NOTE: If we reach here, the conflict session disappeared between
		// FindSessionByRootPath and the snapshot loop (TOCTOU window).
		// Fall through to CreateSession as documented above.
	}

	sessionName := filepath.Base(dir)
	if sessionName == "" || sessionName == "." || sessionName == string(os.PathSeparator) || (len(sessionName) == 2 && sessionName[1] == ':') {
		sessionName = "quick-session"
	}
	sessionName = SanitizeSessionName(sessionName, "quick-session")

	return s.CreateSession(dir, sessionName, CreateSessionOptions{})
}

// CreateSession creates a new session rooted at rootPath.
// If a session with the same name already exists, a numeric suffix (-2, -3, ...)
// is appended automatically (same deduplication as CreateSessionWithWorktree).
// When opts.EnableAgentTeam is true, Agent Teams environment variables are set on the
// session's initial pane so that Claude Code creates team member panes automatically.
func (s *Service) CreateSession(rootPath, sessionName string, opts CreateSessionOptions) (snapshot tmux.SessionSnapshot, retErr error) {
	if s.deps.IsShuttingDown() {
		return tmux.SessionSnapshot{}, errors.New("cannot create session: application is shutting down")
	}
	sessions, _, err := s.requireSessionsAndRouter()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return tmux.SessionSnapshot{}, errors.New("root path is required")
	}
	sessionName = strings.TrimSpace(sessionName)
	sessionName = SanitizeSessionName(sessionName, "session")
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("session name is required")
	}
	sessionName = s.FindAvailableSessionName(sessionName)
	createdName := ""
	sessionMayExist := false
	defer func() {
		if !sessionMayExist {
			return
		}
		if retErr == nil {
			// Emit a snapshot for successful create flows so sidebar/session-list
			// subscribers receive the latest state.
			s.deps.RequestSnapshot(true)
			return
		}
		if createdName != "" {
			slog.Debug("[DEBUG-SESSION] rolling back session after create failure",
				"session", createdName, "error", retErr)
			if rollbackErr := s.RollbackCreatedSession(createdName); rollbackErr != nil {
				retErr = fmt.Errorf("%w (session rollback also failed: %v)", retErr, rollbackErr)
			}
		}
		// Emit snapshot on rollback/error paths so UI state is reconciled with
		// the latest backend state.
		s.deps.RequestSnapshot(true)
	}()

	createdName, err = s.CreateSessionForDirectory(rootPath, sessionName, opts)
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	sessionMayExist = true

	// Set session-level env flags before any additional pane can be created.
	ApplySessionEnvFlags(sessions, createdName, opts.UseClaudeEnv, opts.UsePaneEnv, opts.UseSessionPaneScope)

	// Store git branch metadata for display in the sidebar.
	// NOTE: This enrichment is best-effort. Session creation must continue even if
	// git probing fails because tmux session creation already succeeded.
	EnrichSessionGitMetadata(sessions, createdName, rootPath)

	if err := s.StoreRootPath(createdName, rootPath); err != nil {
		return tmux.SessionSnapshot{}, err
	}
	snapshot, retErr = s.ActivateCreatedSession(createdName)
	return snapshot, retErr
}

// RenameSession renames an existing session.
func (s *Service) RenameSession(oldName, newName string) error {
	if s.deps.IsShuttingDown() {
		return errors.New("cannot rename session: application is shutting down")
	}
	oldName = strings.TrimSpace(oldName)
	if oldName == "" {
		return errors.New("old session name is required")
	}
	newName = strings.TrimSpace(newName)
	newName = SanitizeSessionName(newName, "session")
	if newName == "" {
		return errors.New("new session name is required")
	}
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return err
	}
	if err := sessions.RenameSession(oldName, newName); err != nil {
		return err
	}
	s.deps.McpCleanupSession(oldName)
	if s.GetActiveSessionName() == oldName {
		s.SetActiveSessionName(newName)
	}
	// NOTE: RenameSession does not call RequestSnapshot(true) directly.
	// Instead, the snapshot is triggered automatically by EmitBackendEvent
	// via snapshotEventPolicies["tmux:session-renamed"] = {trigger: true, bypassDebounce: true}.
	// The "tmux:session-renamed" event provides immediate UI feedback (e.g. sidebar rename animation)
	// while the triggered snapshot reconciles the full state.
	s.deps.EmitBackendEvent("tmux:session-renamed", map[string]any{
		"oldName": oldName,
		"newName": newName,
	})
	return nil
}

// KillSession closes one session.
// If deleteWorktree is true and the session has an associated worktree,
// the worktree is removed after the session is destroyed.
// The decision to delete is made by the user via the KillSessionDialog.
//
// Error semantics: a nil error guarantees the tmux session was killed.
// It does NOT guarantee worktree cleanup succeeded — cleanup failures are
// reported via EmitWorktreeCleanupFailure (frontend notification) and slog.Warn.
//
// NOTE: KillSession is intentionally allowed during shutdown because session
// teardown is part of the shutdown sequence. Event emission and snapshot
// operations may silently no-op during shutdown; this is acceptable.
func (s *Service) KillSession(sessionName string, deleteWorktree bool) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return errors.New("session name is required")
	}
	sessions, router, err := s.requireSessionsAndRouter()
	if err != nil {
		return err
	}

	// Capture worktree metadata before destroying the session so it remains
	// available for cleanup and diagnostics even after kill-session succeeds.
	worktreeInfo, wtErr := sessions.GetWorktreeInfo(sessionName)
	if !deleteWorktree && wtErr != nil {
		slog.Debug("[DEBUG-GIT] failed to resolve worktree metadata for killed session (deleteWorktree=false)",
			"session", sessionName, "error", wtErr)
	}

	resp := s.deps.ExecuteRouterRequest(router, ipc.TmuxRequest{
		Command: "kill-session",
		Flags: map[string]any{
			"-t": sessionName,
		},
	})
	if resp.ExitCode != 0 {
		return errors.New(strings.TrimSpace(resp.Stderr))
	}

	// Stop pane buffers that no longer exist (lightweight pane ID lookup).
	existingPanes := sessions.ActivePaneIDs()
	s.deps.CleanupStaleSnapshotState(existingPanes)

	if s.GetActiveSessionName() == sessionName {
		s.SetActiveSessionName("")
	}
	// Release MCP instance state for the destroyed session.
	s.deps.McpCleanupSession(sessionName)
	s.deps.RequestSnapshot(true)

	// Worktree cleanup only when the user explicitly chose to delete.
	if deleteWorktree {
		if wtErr != nil {
			slog.Warn("[WARN-GIT] failed to resolve worktree metadata before cleanup",
				"session", sessionName, "error", wtErr)
			s.EmitWorktreeCleanupFailure(sessionName, "", wtErr)
			// NOTE: Session termination already succeeded. Treat metadata lookup
			// failure as a non-fatal cleanup warning to avoid retry loops that
			// can surface "session not found" after successful kill-session.
			return nil
		} else if worktreeInfo != nil {
			s.CleanupSessionWorktree(WorktreeCleanupParams{
				SessionName: sessionName,
				WtPath:      worktreeInfo.Path,
				RepoPath:    worktreeInfo.RepoPath,
				BranchName:  worktreeInfo.BranchName,
			})
		} else {
			slog.Debug("[DEBUG-SESSION] deleteWorktree requested but session has no worktree metadata",
				"session", sessionName)
		}
	}

	return nil
}

// ===========================================================================
// Session creation internals
// ===========================================================================

// CreateSessionForDirectory creates a tmux session rooted at sessionDir.
//
// Internal step method: exported for cross-package wiring (worktree.Deps).
// External callers should use CreateSession which orchestrates the full
// lifecycle (name dedup → create → env flags → git metadata → activate).
//
// DESIGN NOTE: opts.UsePaneEnv is not directly referenced in this function.
// It is intentionally forwarded to callers who apply it via ApplySessionEnvFlags
// after session creation succeeds. This separation keeps session creation
// (tmux new-session) decoupled from session-level flag storage.
func (s *Service) CreateSessionForDirectory(
	sessionDir,
	sessionName string,
	opts CreateSessionOptions,
) (string, error) {
	router, err := s.deps.RequireRouter()
	if err != nil {
		return "", err
	}
	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-c": sessionDir,
			"-s": sessionName,
		},
	}
	if opts.EnableAgentTeam {
		req.Env = AgentTeamEnvVars(sessionName)
	}

	// Merge claude_env into initial pane env when enabled.
	//
	// DESIGN NOTE (initial pane vs additional pane env merge):
	// Initial pane reads cfg.ClaudeEnv.Vars directly from the config snapshot
	// (fill-only merge: agent_team env takes priority). This is a point-in-time
	// capture at session creation; pane_env is NOT applied to the initial pane.
	//
	// Additional panes use CommandRouter.buildPaneEnvForSession() which reads
	// claudeEnvView() (runtime-updated values via UpdateClaudeEnv) and applies
	// pane_env with overwrite semantics when both flags are enabled.
	//
	// This divergence is intentional:
	//   - Initial pane: config snapshot for deterministic session bootstrap
	//   - Additional panes: latest runtime values for hot-reloaded env changes
	// If the merge rule for initial panes needs to change, update this block
	// independently of buildPaneEnvForSession to avoid unintended side effects.
	if opts.UseClaudeEnv {
		cfg := s.deps.GetConfigSnapshot()
		if cfg.ClaudeEnv != nil && len(cfg.ClaudeEnv.Vars) > 0 {
			if req.Env == nil {
				req.Env = make(map[string]string)
			}
			for k, v := range cfg.ClaudeEnv.Vars {
				if _, exists := req.Env[k]; !exists {
					req.Env[k] = v // fill-only (agent team env takes priority)
				}
			}
		}
	}
	resp := s.deps.ExecuteRouterRequest(router, req)
	if resp.ExitCode != 0 {
		return "", fmt.Errorf("failed to create session: %s", strings.TrimSpace(resp.Stderr))
	}
	createdName := strings.TrimSpace(resp.Stdout)
	if createdName == "" {
		// tmux can occasionally return empty stdout even when session creation
		// succeeded. Attempt cleanup by the requested -s name to avoid orphaning.
		if rollbackErr := s.rollbackSessionByRouter(router, sessionName); rollbackErr != nil {
			return "", fmt.Errorf("failed to create session: empty session name returned by tmux (rollback also failed: %v)", rollbackErr)
		}
		return "", errors.New("failed to create session: empty session name returned by tmux")
	}
	return createdName, nil
}

// rollbackSessionByRouter destroys a session by name via the command router.
func (s *Service) rollbackSessionByRouter(router *tmux.CommandRouter, sessionName string) error {
	resp := s.deps.ExecuteRouterRequest(router, ipc.TmuxRequest{
		Command: "kill-session",
		Flags: map[string]any{
			"-t": sessionName,
		},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("failed to rollback session: %s", strings.TrimSpace(resp.Stderr))
	}
	return nil
}

// StoreRootPath saves the root directory for a newly created session.
//
// Internal step method: exported for cross-package wiring (worktree.Deps).
// External callers should use CreateSession.
func (s *Service) StoreRootPath(sessionName, rootPath string) error {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return err
	}
	if setErr := sessions.SetRootPath(sessionName, rootPath); setErr != nil {
		return fmt.Errorf("failed to set root path for conflict detection: %w", setErr)
	}
	return nil
}

// ActivateCreatedSession finds the just-created session in the snapshot list,
// sets it as the active session, and returns its snapshot.
//
// Internal step method: exported for cross-package wiring (worktree.Deps).
// External callers should use CreateSession.
func (s *Service) ActivateCreatedSession(createdName string) (tmux.SessionSnapshot, error) {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	snapshots := sessions.Snapshot()
	for _, snapshot := range snapshots {
		if snapshot.Name == createdName {
			s.SetActiveSessionName(snapshot.Name)
			return snapshot, nil
		}
	}
	return tmux.SessionSnapshot{}, fmt.Errorf("created session not found: %s", createdName)
}

// RollbackCreatedSession destroys a session on creation failure.
//
// Internal step method: exported for cross-package wiring (worktree.Deps).
// External callers should use CreateSession which handles rollback via defer.
func (s *Service) RollbackCreatedSession(sessionName string) error {
	router, err := s.deps.RequireRouter()
	if err != nil {
		return err
	}
	return s.rollbackSessionByRouter(router, sessionName)
}

// ===========================================================================
// Session lookup
// ===========================================================================

// MaxSessionNameSuffix is the maximum numeric suffix tried when
// deduplicating session names (e.g. name-2 through name-100).
const MaxSessionNameSuffix = 100

// FindAvailableSessionName returns name if no session with that name exists.
// Otherwise it appends -2, -3, ... until a free name is found.
func (s *Service) FindAvailableSessionName(name string) string {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		slog.Debug("[DEBUG-SESSION] FindAvailableSessionName fallback to original name",
			"name", name, "error", err)
		return name
	}
	if !sessions.HasSession(name) {
		return name
	}
	for i := 2; i <= MaxSessionNameSuffix; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !sessions.HasSession(candidate) {
			return candidate
		}
	}
	// Fallback: use timestamp suffix.
	return fmt.Sprintf("%s-%d", name, time.Now().UnixMilli())
}

// FindSessionByRootPath returns the session name that uses the given
// root path, or "" if no active session uses it.
func (s *Service) FindSessionByRootPath(dir string) string {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		// During startup/shutdown, session manager access can fail transiently.
		// Treat this as "no conflict" to avoid blocking session-creation flows.
		slog.Warn("[WARN-SESSION] FindSessionByRootPath: session manager unavailable, assuming no conflict",
			"path", dir, "error", err)
		return ""
	}
	normalizedPath := filepath.Clean(dir)
	snapshots := sessions.Snapshot()
	for _, snap := range snapshots {
		// Worktreeセッションの実際の作業ディレクトリはWorktreePathであり、
		// RootPath(リポジトリパス)ではない。他セッションをブロックしない。
		if snap.Worktree != nil && snap.Worktree.Path != "" {
			continue
		}
		if snap.RootPath != "" && PathsEqualFold(snap.RootPath, normalizedPath) {
			return snap.Name
		}
	}
	return ""
}

// FindSessionByWorktreePath returns the session name that uses the given
// worktree path, or "" if no active session uses it.
func (s *Service) FindSessionByWorktreePath(wtPath string) string {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		// During startup/shutdown, session manager access can fail transiently.
		// Treat this as "no conflict" to avoid blocking worktree-attach flows.
		slog.Warn("[WARN-SESSION] FindSessionByWorktreePath: session manager unavailable, assuming no conflict",
			"path", wtPath, "error", err)
		return ""
	}
	normalizedPath := filepath.Clean(wtPath)
	snapshots := sessions.Snapshot()
	for _, snap := range snapshots {
		if snap.Worktree != nil && snap.Worktree.Path != "" && PathsEqualFold(snap.Worktree.Path, normalizedPath) {
			return snap.Name
		}
	}
	return ""
}

// FindSessionSnapshotByName looks up a session snapshot by session name.
func (s *Service) FindSessionSnapshotByName(sessionName string) (tmux.SessionSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("source session is required")
	}
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	for _, snapshot := range sessions.Snapshot() {
		if snapshot.Name == sessionName {
			return snapshot, nil
		}
	}
	return tmux.SessionSnapshot{}, fmt.Errorf("session %s not found", sessionName)
}

// CheckDirectoryConflict checks whether the given directory is already
// used as the root path by an active session.
// Returns the session name if conflict exists, or "".
func (s *Service) CheckDirectoryConflict(dir string) string {
	return s.FindSessionByRootPath(strings.TrimSpace(dir))
}

// ResolveSessionByCwd resolves a session name from a working directory path.
// It checks both root paths and worktree paths.
func (s *Service) ResolveSessionByCwd(cwd string) (string, error) {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return "", errors.New("cwd is empty")
	}
	if name := s.FindSessionByRootPath(cwd); name != "" {
		return name, nil
	}
	if name := s.FindSessionByWorktreePath(cwd); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("no session found for cwd: %s", cwd)
}

// ResolveSessionDir resolves a directory path for a session.
// When preferWorktree is true, returns the worktree path for worktree sessions (working directory).
// When preferWorktree is false, returns the repo path for worktree sessions (git operations).
// For regular sessions, both modes return root_path.
func (s *Service) ResolveSessionDir(sessionName string, preferWorktree bool) (string, error) {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return "", err
	}

	snapshots := sessions.Snapshot()
	for _, snap := range snapshots {
		if snap.Name != sessionName {
			continue
		}
		if snap.Worktree != nil {
			if preferWorktree && snap.Worktree.Path != "" {
				return snap.Worktree.Path, nil
			}
			if !preferWorktree && snap.Worktree.RepoPath != "" {
				return snap.Worktree.RepoPath, nil
			}
		}
		if snap.RootPath != "" {
			return snap.RootPath, nil
		}
		return "", fmt.Errorf("session %q has no root path configured", sessionName)
	}
	return "", fmt.Errorf("session %q not found", sessionName)
}

// ResolveSessionWorkDir resolves the working directory for a session.
// For worktree sessions, returns the worktree path; otherwise returns root_path.
func (s *Service) ResolveSessionWorkDir(sessionName string) (string, error) {
	return s.ResolveSessionDir(sessionName, true)
}

// ResolveSessionRepoDir resolves the git repository directory for a session.
// For worktree sessions, returns the repo_path (original repository).
// For regular sessions with root_path, returns root_path.
func (s *Service) ResolveSessionRepoDir(sessionName string) (string, error) {
	return s.ResolveSessionDir(sessionName, false)
}

// ListSessions returns current session snapshots.
// Returns nil on error (callers see "zero sessions"). The error is logged at
// Warn level so operators can distinguish "genuinely empty" from "unavailable".
func (s *Service) ListSessions() []tmux.SessionSnapshot {
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		slog.Warn("[WARN-SESSION] ListSessions: session manager unavailable, returning nil",
			"error", err)
		return nil
	}
	return sessions.Snapshot()
}

// GetSessionEnv returns environment variables for one session on demand.
func (s *Service) GetSessionEnv(sessionName string) (map[string]string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, errors.New("session name is required")
	}
	sessions, err := s.deps.RequireSessions()
	if err != nil {
		return nil, err
	}
	return sessions.GetSessionEnv(sessionName)
}

// ===========================================================================
// Worktree cleanup
// ===========================================================================

// CleanupSessionWorktree removes a worktree after session destruction.
// Called only when the user explicitly chose to delete the worktree.
//
// This method does not return an error. Failures are reported via
// EmitWorktreeCleanupFailure (frontend notification) and slog.Warn.
// The caller (KillSession) treats cleanup as best-effort: a nil error from
// KillSession guarantees session kill, not cleanup success.
func (s *Service) CleanupSessionWorktree(params WorktreeCleanupParams) {
	wtPath := strings.TrimSpace(params.WtPath)
	if wtPath == "" {
		slog.Debug("[DEBUG-GIT] skip worktree cleanup: worktree path is empty",
			"session", params.SessionName)
		return
	}
	cfg := s.deps.GetConfigSnapshot()
	repoPath := strings.TrimSpace(params.RepoPath)
	if repoPath == "" {
		err := fmt.Errorf("worktree cleanup skipped: repository path is empty for session %s", params.SessionName)
		slog.Warn("[WARN-GIT] failed to clean up worktree", "session", params.SessionName, "error", err)
		s.EmitWorktreeCleanupFailure(params.SessionName, wtPath, err)
		return
	}

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		slog.Warn("[WARN-GIT] failed to open repo for worktree cleanup", "error", err)
		s.EmitWorktreeCleanupFailure(params.SessionName, wtPath, err)
		return
	}

	// Check for uncommitted changes in the worktree.
	// On any error, skip cleanup to avoid data loss (unless ForceCleanup is set).
	if !cfg.Worktree.ForceCleanup && !s.IsWorktreeCleanForRemoval(wtPath) {
		err := fmt.Errorf("worktree cleanup skipped due to uncommitted changes or status check failure")
		slog.Warn("[WARN-GIT] failed to clean up worktree", "session", params.SessionName, "path", wtPath, "error", err)
		s.EmitWorktreeCleanupFailure(params.SessionName, wtPath, err)
		return
	}

	if err := repo.RemoveWorktree(wtPath); err != nil {
		if !cfg.Worktree.ForceCleanup {
			slog.Warn("[WARN-GIT] failed to remove worktree", "error", err)
			s.EmitWorktreeCleanupFailure(params.SessionName, wtPath, err)
			return
		}
		if fErr := repo.RemoveWorktreeForced(wtPath); fErr != nil {
			slog.Warn("[WARN-GIT] failed to force-remove worktree", "error", fErr)
			s.EmitWorktreeCleanupFailure(params.SessionName, wtPath, fErr)
			return
		}
	}

	gitpkg.PostRemovalCleanup(repo, wtPath)

	s.CleanupOrphanedLocalWorktreeBranch(params.SessionName, repo, params.BranchName)
}

// CleanupOrphanedLocalWorktreeBranch removes a local branch that was created
// solely for a worktree and is no longer needed after worktree removal.
func (s *Service) CleanupOrphanedLocalWorktreeBranch(sessionName string, repo *gitpkg.Repository, branchName string) {
	branchName = strings.TrimSpace(branchName)
	if repo == nil {
		slog.Debug("[DEBUG-GIT] skip orphaned branch cleanup: repository is nil", "branch", branchName)
		return
	}
	if branchName == "" {
		slog.Debug("[DEBUG-GIT] skip orphaned branch cleanup: branch name is empty")
		return
	}
	deleted, err := repo.CleanupLocalBranchIfOrphaned(branchName)
	if err != nil {
		slog.Warn("[WARN-GIT] failed to clean up orphaned local branch",
			"branch", branchName, "error", err)
		s.EmitWorktreeCleanupFailure(sessionName, "", fmt.Errorf("orphaned branch cleanup failed for %s: %w", branchName, err))
		return
	}
	if deleted {
		slog.Debug("[DEBUG-GIT] removed orphaned local branch created for worktree",
			"branch", branchName)
	}
}

// IsWorktreeCleanForRemoval returns true if the worktree has no uncommitted changes.
// On any error (open / check), it returns false to prevent data loss.
func (s *Service) IsWorktreeCleanForRemoval(wtPath string) bool {
	wtRepo, err := gitpkg.Open(wtPath)
	if err != nil {
		slog.Warn("[WARN-GIT] failed to open worktree for change check, skipping cleanup",
			"path", wtPath, "error", err)
		return false
	}
	hasChanges, chkErr := wtRepo.HasUncommittedChanges()
	if chkErr != nil {
		slog.Warn("[WARN-GIT] failed to check uncommitted changes, skipping cleanup",
			"path", wtPath, "error", chkErr)
		return false
	}
	if hasChanges {
		slog.Warn("[WARN-GIT] worktree has uncommitted changes, skipping cleanup",
			"path", wtPath)
		return false
	}
	return true
}

// EmitWorktreeCleanupFailure notifies the frontend that worktree cleanup failed.
func (s *Service) EmitWorktreeCleanupFailure(sessionName, wtPath string, err error) {
	// NOTE: err==nil is defensively handled here to ensure the event payload always
	// contains a non-empty error string. All current callers pass a non-nil err, but
	// this guard prevents silent data loss if a future call site omits the error.
	if err == nil {
		err = fmt.Errorf("unknown worktree cleanup failure")
	}
	// Primary guard: IsShuttingDown is the canonical shutdown check.
	if s.deps.IsShuttingDown() {
		slog.Debug("[DEBUG-SESSION] EmitWorktreeCleanupFailure skipped during shutdown",
			"sessionName", sessionName, "path", wtPath, "error", err)
		return
	}
	// Safety-net guard: RuntimeContext() may return nil independently of
	// IsShuttingDown (e.g. timing between shutdown signal and context teardown).
	ctx := s.deps.RuntimeContext()
	if ctx == nil {
		slog.Warn("[WARN-WORKTREE] EmitWorktreeCleanupFailure: runtime context is nil, event dropped",
			"sessionName", sessionName, "path", wtPath, "error", err)
		return
	}
	s.deps.Emitter.EmitWithContext(ctx, "worktree:cleanup-failed", map[string]any{
		"sessionName": sessionName,
		"path":        wtPath,
		"error":       err.Error(),
	})
}

// OverrideExecuteRouterRequest replaces the router execution function on this
// service instance and returns a restore function.
// Exported for cross-package test stubbing where the function must be changed
// after service construction.
//
// WARNING: NOT thread-safe. Tests that use this method MUST NOT call
// t.Parallel() on the same Service instance. Mutating the deps field is not
// concurrent-safe with Service method calls.
func (s *Service) OverrideExecuteRouterRequest(fn func(*tmux.CommandRouter, ipc.TmuxRequest) ipc.TmuxResponse) func() {
	orig := s.deps.ExecuteRouterRequest
	s.deps.ExecuteRouterRequest = fn
	return func() { s.deps.ExecuteRouterRequest = orig }
}
