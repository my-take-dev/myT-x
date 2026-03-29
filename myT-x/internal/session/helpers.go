package session

import (
	"log/slog"
	"path/filepath"
	"strings"

	gitpkg "myT-x/internal/git"
	"myT-x/internal/tmux"
)

// SanitizeSessionName delegates to the shared tmux utility.
// Collapses runs of characters invalid in tmux session names (dots, colons)
// into a single hyphen.
func SanitizeSessionName(name, fallback string) string {
	return tmux.SanitizeSessionName(name, fallback)
}

// AgentTeamEnvVars returns the environment variables that signal Claude Code
// to enable Agent Teams mode. The caller is responsible for passing these
// through the IPC request's Env field.
//
// NOTE: MYTX_SESSION is NOT included here — it is universally injected by
// addTmuxEnvironment (Layer 5) for ALL panes, not just Agent Teams panes.
func AgentTeamEnvVars(teamName string) map[string]string {
	return map[string]string{
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1",
		"CLAUDE_CODE_TEAM_NAME":                teamName,
		"CLAUDE_CODE_AGENT_ID":                 "lead",
		"CLAUDE_CODE_AGENT_TYPE":               "lead",
	}
}

// ApplySessionEnvFlags sets session-level UseClaudeEnv and UsePaneEnv flags
// on the SessionManager. These flags control whether additional panes inherit
// claude_env / pane_env variables via buildPaneEnvForSession.
//
// IMPORTANT: Every session creation path (CreateSession, CreateSessionWithWorktree,
// CreateSessionWithExistingWorktree) must call this function after
// createSessionForDirectory succeeds. When adding a new session creation path,
// ensure ApplySessionEnvFlags is called to avoid silent flag-loss.
//
// NOTE: Failure is non-fatal (Warn only). The only realistic failure case is
// session-name mismatch in SessionManager, which cannot happen here because
// the session was just created successfully by createSessionForDirectory.
// Aborting the entire session creation for a flag-storage failure would be
// disproportionate; the session remains fully functional without these flags
// (additional panes simply won't inherit the configured env).
func ApplySessionEnvFlags(sm *tmux.SessionManager, sessionName string, useClaudeEnv, usePaneEnv, useSessionPaneScope bool) {
	if useClaudeEnv {
		if setErr := sm.SetUseClaudeEnv(sessionName, useClaudeEnv); setErr != nil {
			slog.Warn("[WARN-ENV] failed to set UseClaudeEnv flag", "session", sessionName, "error", setErr)
		}
	}
	if usePaneEnv {
		if setErr := sm.SetUsePaneEnv(sessionName, usePaneEnv); setErr != nil {
			slog.Warn("[WARN-ENV] failed to set UsePaneEnv flag", "session", sessionName, "error", setErr)
		}
	}
	if useSessionPaneScope {
		if setErr := sm.SetUseSessionPaneScope(sessionName, useSessionPaneScope); setErr != nil {
			slog.Warn("[WARN-ENV] failed to set UseSessionPaneScope flag", "session", sessionName, "error", setErr)
		}
	}
}

// EnrichSessionGitMetadata probes the rootPath for git information and stores
// branch metadata on the session for sidebar display. This is best-effort:
// any failure is logged but does not abort session creation.
func EnrichSessionGitMetadata(sessions *tmux.SessionManager, sessionName, rootPath string) {
	if !gitpkg.IsGitRepository(rootPath) {
		return
	}
	repo, err := gitpkg.Open(rootPath)
	if err != nil {
		slog.Warn("[WARN-SESSION] failed to open git repo for session metadata",
			"path", rootPath, "error", err)
		return
	}
	branch, err := repo.CurrentBranch()
	if err != nil {
		slog.Warn("[WARN-SESSION] failed to read git branch for session metadata",
			"path", rootPath, "error", err)
		return
	}
	// wtPath="" signals "no worktree, just tracking git info".
	if setErr := sessions.SetWorktreeInfo(sessionName, &tmux.SessionWorktreeInfo{
		RepoPath:   rootPath,
		BranchName: branch,
	}); setErr != nil {
		slog.Warn("[WARN-SESSION] failed to store git info for session",
			"session", sessionName, "error", setErr)
	}
}

// PathsEqualFold compares two file paths case-insensitively after normalization.
// Suitable for Windows path comparison where case is insignificant.
func PathsEqualFold(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// ResolveSessionDirectory returns the effective working directory for the session.
// Worktree path takes priority over RootPath.
// Both paths are TrimSpace'd symmetrically to avoid passing whitespace-padded
// paths to explorer.exe or other consumers.
func ResolveSessionDirectory(s tmux.SessionSnapshot) string {
	if s.Worktree != nil {
		if p := strings.TrimSpace(s.Worktree.Path); p != "" {
			return p
		}
	}
	return strings.TrimSpace(s.RootPath)
}
