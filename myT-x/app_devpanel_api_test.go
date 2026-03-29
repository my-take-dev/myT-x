package main

import (
	"testing"

	"myT-x/internal/tmux"
)

func TestResolveSessionWorkDir(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		rootPath    string
		worktree    *tmux.SessionWorktreeInfo
		wantErr     bool
		wantPath    string
	}{
		{
			name:        "returns root_path for regular session",
			sessionName: "regular",
			rootPath:    "/projects/myapp",
			wantPath:    "/projects/myapp",
		},
		{
			name:        "missing session returns error",
			sessionName: "nonexistent",
			wantErr:     true,
		},
		{
			name:        "session without root_path returns error",
			sessionName: "no-root",
			rootPath:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}
			mgr := tmux.NewSessionManager()
			if tt.sessionName == "regular" || tt.sessionName == "no-root" {
				if _, _, err := mgr.CreateSession(tt.sessionName, "bash", 80, 24); err != nil {
					t.Fatalf("failed to create session: %v", err)
				}
				if tt.rootPath != "" {
					if err := mgr.SetRootPath(tt.sessionName, tt.rootPath); err != nil {
						t.Fatalf("failed to set root path: %v", err)
					}
				}
				if tt.worktree != nil {
					if err := mgr.SetWorktreeInfo(tt.sessionName, tt.worktree); err != nil {
						t.Fatalf("failed to set worktree info: %v", err)
					}
				}
			}
			app.sessions = mgr
			app.sessionService = newSessionServiceForTest(app)

			result, err := app.sessionService.ResolveSessionWorkDir(tt.sessionName)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%s)", result)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantPath {
				t.Fatalf("result = %q, want %q", result, tt.wantPath)
			}
		})
	}
}
