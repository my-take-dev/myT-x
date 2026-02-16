package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestSplitWindowWorkDirFallback(t *testing.T) {
	type rootPathMode int
	const (
		rootPathUnset rootPathMode = iota
		rootPathMissing
		rootPathWhitespace
	)

	type workDirMode int
	const (
		workDirOmitted workDirMode = iota
		workDirEmpty
		workDirWhitespace
		workDirValid
	)

	tests := []struct {
		name         string
		rootPath     rootPathMode
		workDir      workDirMode
		wantExitCode int
		wantPaneOne  bool
	}{
		{
			name:         "omitted workDir and unset session root fall back to host cwd",
			rootPath:     rootPathUnset,
			workDir:      workDirOmitted,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
		{
			name:         "omitted workDir falls back to session workDir",
			rootPath:     rootPathMissing,
			workDir:      workDirOmitted,
			wantExitCode: 1,
			wantPaneOne:  false,
		},
		{
			name:         "whitespace workDir falls back to session workDir",
			rootPath:     rootPathMissing,
			workDir:      workDirWhitespace,
			wantExitCode: 1,
			wantPaneOne:  false,
		},
		{
			name:         "explicit workDir is not overwritten by fallback",
			rootPath:     rootPathMissing,
			workDir:      workDirValid,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
		{
			name:         "empty workDir and empty session fallback uses host cwd",
			rootPath:     rootPathWhitespace,
			workDir:      workDirEmpty,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			missingDir := filepath.Join(t.TempDir(), "missing-workdir")
			validDir := os.TempDir()

			switch tt.rootPath {
			case rootPathMissing:
				if err := sessions.SetRootPath("demo", missingDir); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			case rootPathWhitespace:
				if err := sessions.SetRootPath("demo", "   "); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			case rootPathUnset:
				// Keep session root path empty.
			default:
				t.Fatalf("unexpected rootPath mode: %d", tt.rootPath)
			}

			flags := map[string]any{
				"-t": "demo:0",
				"-h": true,
			}
			switch tt.workDir {
			case workDirOmitted:
				// Simulates GUI split path where -c is not provided.
			case workDirEmpty:
				flags["-c"] = ""
			case workDirWhitespace:
				flags["-c"] = "   "
			case workDirValid:
				flags["-c"] = validDir
			default:
				t.Fatalf("unexpected workDir mode: %d", tt.workDir)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "split-window",
				Flags:   flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("split-window exit code = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.wantExitCode == 0 {
				paneID := strings.TrimSpace(resp.Stdout)
				if !strings.HasPrefix(paneID, "%") {
					t.Fatalf("split-window stdout = %q, want pane id", resp.Stdout)
				}
			} else if strings.TrimSpace(resp.Stderr) == "" {
				t.Fatal("split-window failure returned empty stderr")
			}

			if got := sessions.HasPane("%1"); got != tt.wantPaneOne {
				t.Fatalf("HasPane(%%1) = %v, want %v", got, tt.wantPaneOne)
			}
		})
	}
}

func TestSplitWindowWorkDirFallbackUsesWorktreePath(t *testing.T) {
	sessions := NewSessionManager()
	defer sessions.Close()

	router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})
	if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	missingRoot := filepath.Join(t.TempDir(), "missing-root")
	if err := sessions.SetRootPath("demo", missingRoot); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}

	worktreeDir := os.TempDir()
	if err := sessions.SetWorktreeInfo("demo", &SessionWorktreeInfo{Path: worktreeDir}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	resp := router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags: map[string]any{
			"-t": "demo:0",
			"-h": true,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("split-window exit code = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	paneID := strings.TrimSpace(resp.Stdout)
	if !strings.HasPrefix(paneID, "%") {
		t.Fatalf("split-window stdout = %q, want pane id", resp.Stdout)
	}
	if !sessions.HasPane("%1") {
		t.Fatal("HasPane(%1) = false, want true")
	}
}
