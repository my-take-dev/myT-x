package session

import (
	"testing"

	"myT-x/internal/tmux"
)

func TestSanitizeSessionName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
		want     string
	}{
		{name: "clean name", input: "my-session", fallback: "default", want: "my-session"},
		{name: "empty returns fallback", input: "", fallback: "quick-session", want: "quick-session"},
		{name: "whitespace only preserved", input: "   ", fallback: "default", want: "   "},
		{name: "dots collapsed", input: "a.b.c", fallback: "default", want: "a-b-c"},
		{name: "colons collapsed", input: "a:b:c", fallback: "default", want: "a-b-c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSessionName(tt.input, tt.fallback)
			if got != tt.want {
				t.Errorf("SanitizeSessionName(%q, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestAgentTeamEnvVars(t *testing.T) {
	vars := AgentTeamEnvVars("team-1")
	requiredKeys := []string{
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS",
		"CLAUDE_CODE_TEAM_NAME",
		"CLAUDE_CODE_AGENT_ID",
		"CLAUDE_CODE_AGENT_TYPE",
	}
	for _, key := range requiredKeys {
		if _, ok := vars[key]; !ok {
			t.Errorf("AgentTeamEnvVars missing required key %q", key)
		}
	}
	if vars["CLAUDE_CODE_TEAM_NAME"] != "team-1" {
		t.Errorf("CLAUDE_CODE_TEAM_NAME = %q, want %q", vars["CLAUDE_CODE_TEAM_NAME"], "team-1")
	}
	// MYTX_SESSION must NOT be included (managed by addTmuxEnvironment).
	if _, ok := vars["MYTX_SESSION"]; ok {
		t.Fatal("MYTX_SESSION must not be in AgentTeamEnvVars")
	}
}

func TestPathsEqualFold(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "identical", a: "C:/foo/bar", b: "C:/foo/bar", want: true},
		{name: "case difference", a: "C:/Foo/Bar", b: "c:/foo/bar", want: true},
		{name: "trailing separator", a: "C:/foo/bar/", b: "C:/foo/bar", want: true},
		{name: "different paths", a: "C:/foo", b: "C:/bar", want: false},
		{name: "empty vs non-empty", a: "", b: "C:/foo", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PathsEqualFold(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("PathsEqualFold(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestResolveSessionDirectory(t *testing.T) {
	tests := []struct {
		name     string
		snapshot tmux.SessionSnapshot
		want     string
	}{
		{
			name:     "worktree path takes priority",
			snapshot: tmux.SessionSnapshot{RootPath: "/repo", Worktree: &tmux.SessionWorktreeInfo{Path: "/wt/path"}},
			want:     "/wt/path",
		},
		{
			name:     "empty worktree path falls back to root",
			snapshot: tmux.SessionSnapshot{RootPath: "/repo", Worktree: &tmux.SessionWorktreeInfo{Path: ""}},
			want:     "/repo",
		},
		{
			name:     "nil worktree uses root",
			snapshot: tmux.SessionSnapshot{RootPath: "/repo"},
			want:     "/repo",
		},
		{
			name:     "no path returns empty",
			snapshot: tmux.SessionSnapshot{},
			want:     "",
		},
		{
			name:     "whitespace trimmed from worktree path",
			snapshot: tmux.SessionSnapshot{Worktree: &tmux.SessionWorktreeInfo{Path: "  /wt/path  "}},
			want:     "/wt/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSessionDirectory(tt.snapshot)
			if got != tt.want {
				t.Errorf("ResolveSessionDirectory() = %q, want %q", got, tt.want)
			}
		})
	}
}
