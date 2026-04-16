package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
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

func newDevPanelAppForTest(t *testing.T) (*App, string) {
	t.Helper()

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)

	rootPath := t.TempDir()
	if _, _, err := app.sessions.CreateSession("session-a", "bash", 80, 24); err != nil {
		t.Fatalf("CreateSession(session-a): %v", err)
	}
	if err := app.sessions.SetRootPath("session-a", rootPath); err != nil {
		t.Fatalf("SetRootPath(session-a): %v", err)
	}
	return app, rootPath
}

func devPanelSessionKeyForTest(t *testing.T, app *App, sessionName string) string {
	t.Helper()

	sessionSnapshot, err := app.sessionService.FindSessionSnapshotByName(sessionName)
	if err != nil {
		t.Fatalf("FindSessionSnapshotByName(%q): %v", sessionName, err)
	}
	return buildSessionKey(sessionSnapshot.Name, sessionSnapshot.ID)
}

func TestDevPanelFileMutationWrappers(t *testing.T) {
	app, rootPath := newDevPanelAppForTest(t)
	sessionKey := devPanelSessionKeyForTest(t, app, "session-a")

	if err := app.DevPanelCreateDirectory(sessionKey, "nested"); err != nil {
		t.Fatalf("DevPanelCreateDirectory() error = %v", err)
	}

	writeResult, err := app.DevPanelWriteFile(sessionKey, "nested/file.txt", "hello world")
	if err != nil {
		t.Fatalf("DevPanelWriteFile() error = %v", err)
	}
	if writeResult.Path != "nested/file.txt" {
		t.Fatalf("WriteFileResult.Path = %q, want %q", writeResult.Path, "nested/file.txt")
	}

	meta, err := app.DevPanelGetFileInfo("session-a", "nested/file.txt")
	if err != nil {
		t.Fatalf("DevPanelGetFileInfo() error = %v", err)
	}
	if meta.IsDir {
		t.Fatal("DevPanelGetFileInfo() returned IsDir=true for a file")
	}
	if meta.Size != int64(len("hello world")) {
		t.Fatalf("FileMetadata.Size = %d, want %d", meta.Size, len("hello world"))
	}

	content, err := os.ReadFile(filepath.Join(rootPath, "nested", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile(nested/file.txt) error = %v", err)
	}
	if string(content) != "hello world" {
		t.Fatalf("file content = %q, want %q", string(content), "hello world")
	}
}

func TestDevPanelWatcherWrappers(t *testing.T) {
	app, _ := newDevPanelAppForTest(t)
	sessionKey := devPanelSessionKeyForTest(t, app, "session-a")

	if err := app.DevPanelStartWatcher(sessionKey); err != nil {
		t.Fatalf("DevPanelStartWatcher() error = %v", err)
	}
	if err := app.DevPanelStopWatcher(sessionKey); err != nil {
		t.Fatalf("DevPanelStopWatcher() error = %v", err)
	}
}

func TestDevPanelReadBinary(t *testing.T) {
	app, rootPath := newDevPanelAppForTest(t)
	original := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	targetPath := filepath.Join(rootPath, "nested", "image.png")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(targetPath), err)
	}
	if err := os.WriteFile(targetPath, original, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", targetPath, err)
	}

	result, err := app.DevPanelReadBinary("session-a", "nested/image.png")
	if err != nil {
		t.Fatalf("DevPanelReadBinary() error = %v", err)
	}
	if result.Path != "nested/image.png" {
		t.Fatalf("BinaryFileContent.Path = %q, want %q", result.Path, "nested/image.png")
	}
	if result.Mime != "image/png" {
		t.Fatalf("BinaryFileContent.Mime = %q, want %q", result.Mime, "image/png")
	}

	decoded, decodeErr := base64.StdEncoding.DecodeString(result.Data)
	if decodeErr != nil {
		t.Fatalf("DecodeString() error = %v", decodeErr)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatal("decoded binary content mismatch")
	}
}

func TestDevPanelMutationWrappersRejectUnknownSessionKey(t *testing.T) {
	app, _ := newDevPanelAppForTest(t)

	if _, err := app.DevPanelCreateFile("session-a:9999", "nested/file.txt"); err == nil {
		t.Fatal("DevPanelCreateFile() error = nil, want session key validation error")
	}
}
