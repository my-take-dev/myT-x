package tmux

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetWorktreeInfoReturnsCopy(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	original := &SessionWorktreeInfo{
		Path:       `C:\Projects\repo.wt\feature`,
		RepoPath:   `C:\Projects\repo`,
		BranchName: "feature",
		BaseBranch: "main",
		IsDetached: false,
	}
	if err := manager.SetWorktreeInfo("demo", original); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	got, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetWorktreeInfo() returned nil, want non-nil")
	}
	if got == original {
		t.Fatal("GetWorktreeInfo() returned same pointer, want copy")
	}
	if got.Path != original.Path || got.RepoPath != original.RepoPath || got.BranchName != original.BranchName ||
		got.BaseBranch != original.BaseBranch || got.IsDetached != original.IsDetached {
		t.Fatalf("GetWorktreeInfo() = %+v, want %+v", got, original)
	}

	original.Path = `C:\mutated`
	afterMutatingOriginal, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if afterMutatingOriginal.Path != `C:\Projects\repo.wt\feature` {
		t.Fatalf("stored worktree path mutated via input pointer: %q", afterMutatingOriginal.Path)
	}

	got.Path = `C:\mutated-from-return`
	afterMutatingReturn, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if afterMutatingReturn.Path != `C:\Projects\repo.wt\feature` {
		t.Fatalf("stored worktree path mutated via returned pointer: %q", afterMutatingReturn.Path)
	}
}

func TestSetWorktreeInfoNilClearsMetadata(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if err := manager.SetWorktreeInfo("demo", &SessionWorktreeInfo{
		Path:       `C:\Projects\repo.wt\feature`,
		RepoPath:   `C:\Projects\repo`,
		BranchName: "feature",
		BaseBranch: "main",
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}
	if err := manager.SetWorktreeInfo("demo", nil); err != nil {
		t.Fatalf("SetWorktreeInfo(nil) error = %v", err)
	}

	got, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetWorktreeInfo() = %+v, want nil after clear", got)
	}
}

func TestSetWorktreeInfoEmptyStructClearsMetadata(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if err := manager.SetWorktreeInfo("demo", &SessionWorktreeInfo{
		Path:       `C:\Projects\repo.wt\feature`,
		RepoPath:   `C:\Projects\repo`,
		BranchName: "feature",
		BaseBranch: "main",
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}
	if err := manager.SetWorktreeInfo("demo", &SessionWorktreeInfo{}); err != nil {
		t.Fatalf("SetWorktreeInfo(empty) error = %v", err)
	}

	got, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetWorktreeInfo() = %+v, want nil after empty update", got)
	}
}

func TestSetWorktreeInfoKeepsDetachedOrNonEmptyMetadata(t *testing.T) {
	tests := []struct {
		name string
		info SessionWorktreeInfo
	}{
		{
			name: "detached true is preserved",
			info: SessionWorktreeInfo{IsDetached: true},
		},
		{
			name: "single field is preserved",
			info: SessionWorktreeInfo{RepoPath: `C:\Projects\repo`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSessionManager()
			if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
			if err := manager.SetWorktreeInfo("demo", &tt.info); err != nil {
				t.Fatalf("SetWorktreeInfo() error = %v", err)
			}

			got, err := manager.GetWorktreeInfo("demo")
			if err != nil {
				t.Fatalf("GetWorktreeInfo() error = %v", err)
			}
			if got == nil {
				t.Fatal("GetWorktreeInfo() returned nil, want non-nil")
			}
		})
	}
}

func TestSnapshotReturnsWorktreeCopy(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := manager.SetWorktreeInfo("demo", &SessionWorktreeInfo{
		Path:       `C:\Projects\repo.wt\feature`,
		RepoPath:   `C:\Projects\repo`,
		BranchName: "feature",
		BaseBranch: "main",
		IsDetached: false,
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	snapshots := manager.Snapshot()
	if len(snapshots) != 1 || snapshots[0].Worktree == nil {
		t.Fatalf("Snapshot() did not include worktree: %+v", snapshots)
	}
	snapshots[0].Worktree.Path = `C:\mutated`

	afterMutation, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if afterMutation.Path != `C:\Projects\repo.wt\feature` {
		t.Fatalf("worktree path mutated through snapshot copy: %q", afterMutation.Path)
	}
}

func TestSetWorktreeInfoValidatesSessionName(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "session name is required"},
		{name: "whitespace", input: "   ", wantErr: "session name is required"},
		{name: "missing", input: "missing", wantErr: "session not found: missing"},
		{name: "colon target resolves", input: "demo:0", wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SetWorktreeInfo(tt.input, nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("SetWorktreeInfo() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestGetWorktreeInfoValidatesSessionName(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "session name is required"},
		{name: "whitespace", input: "   ", wantErr: "session name is required"},
		{name: "missing", input: "missing", wantErr: "session not found: missing"},
		{name: "colon target resolves", input: "demo:0", wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.GetWorktreeInfo(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GetWorktreeInfo() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSetRootPathValidatesSessionName(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "session name is required"},
		{name: "whitespace", input: "   ", wantErr: "session name is required"},
		{name: "missing", input: "missing", wantErr: "session not found: missing"},
		{name: "colon target resolves", input: "demo:0", wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SetRootPath(tt.input, `C:\Projects\repo`)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("SetRootPath() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSessionWorktreeInfoJSONIncludesDetachedFalse(t *testing.T) {
	raw, err := json.Marshal(SessionWorktreeInfo{
		Path:       `C:\Projects\repo.wt\feature`,
		IsDetached: false,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(raw), `"is_detached":false`) {
		t.Fatalf("JSON = %s, want is_detached=false field", string(raw))
	}
}

func TestSetAgentTeam(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		isAgentTeam bool
		wantErr     string
	}{
		{
			name:        "set true",
			target:      "demo",
			isAgentTeam: true,
		},
		{
			name:        "set false",
			target:      "demo:0",
			isAgentTeam: false,
		},
		{
			name:        "missing session",
			target:      "missing",
			isAgentTeam: true,
			wantErr:     "session not found: missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSessionManager()
			if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			err := manager.SetAgentTeam(tt.target, tt.isAgentTeam)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("SetAgentTeam() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetAgentTeam() error = %v", err)
			}

			snapshots := manager.Snapshot()
			if len(snapshots) != 1 {
				t.Fatalf("Snapshot() session count = %d, want 1", len(snapshots))
			}
			if snapshots[0].IsAgentTeam != tt.isAgentTeam {
				t.Fatalf("Snapshot()[0].IsAgentTeam = %v, want %v", snapshots[0].IsAgentTeam, tt.isAgentTeam)
			}
		})
	}
}

func TestGetPaneContextSnapshot(t *testing.T) {
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane.Env["FOO"] = "bar"
	pane.Title = "editor"

	snapshot, err := manager.GetPaneContextSnapshot(pane.ID)
	if err != nil {
		t.Fatalf("GetPaneContextSnapshot() error = %v", err)
	}
	if snapshot.SessionID != session.ID {
		t.Fatalf("SessionID = %d, want %d", snapshot.SessionID, session.ID)
	}
	if snapshot.SessionName != session.Name {
		t.Fatalf("SessionName = %q, want %q", snapshot.SessionName, session.Name)
	}
	if snapshot.Title != pane.Title {
		t.Fatalf("Title = %q, want %q", snapshot.Title, pane.Title)
	}
	if snapshot.Env["FOO"] != "bar" {
		t.Fatalf("Env[FOO] = %q, want bar", snapshot.Env["FOO"])
	}
	if snapshot.Layout == nil {
		t.Fatal("Layout is nil, want non-nil")
	}

	snapshot.Env["FOO"] = "mutated"
	if pane.Env["FOO"] != "bar" {
		t.Fatalf("pane env mutated through snapshot copy: %q", pane.Env["FOO"])
	}

	snapshot.Layout.PaneID = 999
	if pane.Window != nil && pane.Window.Layout != nil && pane.Window.Layout.PaneID == 999 {
		t.Fatal("window layout mutated through snapshot copy")
	}

	if _, err := manager.GetPaneContextSnapshot(999); err == nil || !strings.Contains(err.Error(), "pane not found: %999") {
		t.Fatalf("GetPaneContextSnapshot(999) error = %v, want pane not found", err)
	}
}

func TestGetPaneContextSnapshotSessionWorkDir(t *testing.T) {
	type arrangeFunc func(*testing.T, *SessionManager, string)

	tests := []struct {
		name    string
		arrange arrangeFunc
		wantDir string
	}{
		{
			name:    "empty when root and worktree are unset",
			wantDir: "",
		},
		{
			name: "uses root path for regular sessions",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetRootPath(sessionName, `C:\Projects\myapp`); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			},
			wantDir: `C:\Projects\myapp`,
		},
		{
			name: "trimmed root path is returned",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetRootPath(sessionName, "  C:\\Projects\\myapp  "); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			},
			wantDir: `C:\Projects\myapp`,
		},
		{
			name: "worktree path takes precedence over root path",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetRootPath(sessionName, `C:\Projects\myapp`); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
				if err := manager.SetWorktreeInfo(sessionName, &SessionWorktreeInfo{
					Path:     `C:\Projects\myapp.wt\feature`,
					RepoPath: `C:\Projects\myapp`,
				}); err != nil {
					t.Fatalf("SetWorktreeInfo() error = %v", err)
				}
			},
			wantDir: `C:\Projects\myapp.wt\feature`,
		},
		{
			name: "whitespace worktree path falls back to root path",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetRootPath(sessionName, `C:\Projects\myapp`); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
				if err := manager.SetWorktreeInfo(sessionName, &SessionWorktreeInfo{
					Path:     "   ",
					RepoPath: `C:\Projects\myapp`,
				}); err != nil {
					t.Fatalf("SetWorktreeInfo() error = %v", err)
				}
			},
			wantDir: `C:\Projects\myapp`,
		},
		{
			name: "whitespace root path is treated as empty",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetRootPath(sessionName, "   "); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			},
			wantDir: "",
		},
		{
			name: "worktree path is used when root path is unset",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetWorktreeInfo(sessionName, &SessionWorktreeInfo{
					Path:     `C:\Projects\myapp.wt\feature`,
					RepoPath: `C:\Projects\myapp`,
				}); err != nil {
					t.Fatalf("SetWorktreeInfo() error = %v", err)
				}
			},
			wantDir: `C:\Projects\myapp.wt\feature`,
		},
		{
			name: "trimmed worktree path is returned",
			arrange: func(t *testing.T, manager *SessionManager, sessionName string) {
				t.Helper()
				if err := manager.SetWorktreeInfo(sessionName, &SessionWorktreeInfo{
					Path:     "  C:\\Projects\\myapp.wt\\feature  ",
					RepoPath: `C:\Projects\myapp`,
				}); err != nil {
					t.Fatalf("SetWorktreeInfo() error = %v", err)
				}
			},
			wantDir: `C:\Projects\myapp.wt\feature`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSessionManager()
			session, pane, err := manager.CreateSession("demo", "0", 120, 40)
			if err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
			if tt.arrange != nil {
				tt.arrange(t, manager, session.Name)
			}

			snapshot, err := manager.GetPaneContextSnapshot(pane.ID)
			if err != nil {
				t.Fatalf("GetPaneContextSnapshot() error = %v", err)
			}
			if snapshot.SessionWorkDir != tt.wantDir {
				t.Fatalf("SessionWorkDir = %q, want %q", snapshot.SessionWorkDir, tt.wantDir)
			}
		})
	}
}

func TestHasPane(t *testing.T) {
	manager := NewSessionManager()
	_, pane, err := manager.CreateSession("demo", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tests := []struct {
		name   string
		input  string
		exists bool
	}{
		{name: "existing pane", input: pane.IDString(), exists: true},
		{name: "existing pane with spaces", input: " " + pane.IDString() + " ", exists: true},
		{name: "missing pane", input: "%999", exists: false},
		{name: "invalid format", input: "999", exists: false},
		{name: "empty", input: "", exists: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := manager.HasPane(tt.input); got != tt.exists {
				t.Fatalf("HasPane(%q) = %v, want %v", tt.input, got, tt.exists)
			}
		})
	}
}

func TestSessionWorktreeInfoIsEmptyBoundaries(t *testing.T) {
	var nilInfo *SessionWorktreeInfo
	if !nilInfo.IsEmpty() {
		t.Fatal("nil receiver should be empty")
	}

	tests := []struct {
		name string
		info SessionWorktreeInfo
		want bool
	}{
		{name: "all zero", info: SessionWorktreeInfo{}, want: true},
		{name: "detached only", info: SessionWorktreeInfo{IsDetached: true}, want: false},
		{name: "whitespace only path", info: SessionWorktreeInfo{Path: "   "}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.IsEmpty(); got != tt.want {
				t.Fatalf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionWorktreeInfoIsWorktreeSessionBoundaries(t *testing.T) {
	var nilInfo *SessionWorktreeInfo
	if nilInfo.IsWorktreeSession() {
		t.Fatal("nil receiver should not be a worktree session")
	}

	tests := []struct {
		name string
		info SessionWorktreeInfo
		want bool
	}{
		{name: "empty path", info: SessionWorktreeInfo{}, want: false},
		{name: "whitespace path", info: SessionWorktreeInfo{Path: "   "}, want: false},
		{name: "valid path", info: SessionWorktreeInfo{Path: `C:\Projects\repo.wt\feature`}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.IsWorktreeSession(); got != tt.want {
				t.Fatalf("IsWorktreeSession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetWorktreeInfoTrimsWhitespace(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if err := manager.SetWorktreeInfo("demo", &SessionWorktreeInfo{
		Path:       "   ",
		RepoPath:   "  C:\\Projects\\repo  ",
		BranchName: " feature  ",
		BaseBranch: "  ",
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	info, err := manager.GetWorktreeInfo("demo")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetWorktreeInfo() returned nil, want non-nil")
	}
	if info.Path != "" {
		t.Fatalf("Path = %q, want empty", info.Path)
	}
	if info.RepoPath != `C:\Projects\repo` {
		t.Fatalf("RepoPath = %q, want trimmed value", info.RepoPath)
	}
	if info.BranchName != "feature" {
		t.Fatalf("BranchName = %q, want trimmed value", info.BranchName)
	}
	if info.BaseBranch != "" {
		t.Fatalf("BaseBranch = %q, want empty", info.BaseBranch)
	}
}

func TestWorktreeInfoEqual(t *testing.T) {
	base := &SessionWorktreeInfo{
		Path:       `C:\Projects\repo.wt\feature`,
		RepoPath:   `C:\Projects\repo`,
		BranchName: "feature",
		BaseBranch: "main",
		IsDetached: false,
	}

	tests := []struct {
		name  string
		left  *SessionWorktreeInfo
		right *SessionWorktreeInfo
		want  bool
	}{
		{
			name:  "both nil",
			left:  nil,
			right: nil,
			want:  true,
		},
		{
			name:  "nil and non-nil",
			left:  nil,
			right: base,
			want:  false,
		},
		{
			name: "equal values",
			left: &SessionWorktreeInfo{
				Path:       base.Path,
				RepoPath:   base.RepoPath,
				BranchName: base.BranchName,
				BaseBranch: base.BaseBranch,
				IsDetached: base.IsDetached,
			},
			right: &SessionWorktreeInfo{
				Path:       base.Path,
				RepoPath:   base.RepoPath,
				BranchName: base.BranchName,
				BaseBranch: base.BaseBranch,
				IsDetached: base.IsDetached,
			},
			want: true,
		},
		{
			name: "path differs",
			left: base,
			right: &SessionWorktreeInfo{
				Path:       `C:\Projects\repo.wt\feature-b`,
				RepoPath:   base.RepoPath,
				BranchName: base.BranchName,
				BaseBranch: base.BaseBranch,
				IsDetached: base.IsDetached,
			},
			want: false,
		},
		{
			name: "repo path differs",
			left: base,
			right: &SessionWorktreeInfo{
				Path:       base.Path,
				RepoPath:   `D:\repo`,
				BranchName: base.BranchName,
				BaseBranch: base.BaseBranch,
				IsDetached: base.IsDetached,
			},
			want: false,
		},
		{
			name: "branch differs",
			left: base,
			right: &SessionWorktreeInfo{
				Path:       base.Path,
				RepoPath:   base.RepoPath,
				BranchName: "feature-b",
				BaseBranch: base.BaseBranch,
				IsDetached: base.IsDetached,
			},
			want: false,
		},
		{
			name: "base branch differs",
			left: base,
			right: &SessionWorktreeInfo{
				Path:       base.Path,
				RepoPath:   base.RepoPath,
				BranchName: base.BranchName,
				BaseBranch: "release",
				IsDetached: base.IsDetached,
			},
			want: false,
		},
		{
			name: "detached differs",
			left: base,
			right: &SessionWorktreeInfo{
				Path:       base.Path,
				RepoPath:   base.RepoPath,
				BranchName: base.BranchName,
				BaseBranch: base.BaseBranch,
				IsDetached: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worktreeInfoEqual(tt.left, tt.right); got != tt.want {
				t.Fatalf("worktreeInfoEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
