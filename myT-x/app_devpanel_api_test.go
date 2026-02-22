package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// newTestAppWithSession creates an App with a session manager containing
// a single session with the specified rootPath.
func newTestAppWithSession(t *testing.T, sessionName, rootPath string) *App {
	t.Helper()
	app := &App{}
	mgr := tmux.NewSessionManager()
	if _, _, err := mgr.CreateSession(sessionName, "bash", 80, 24); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	if rootPath != "" {
		if err := mgr.SetRootPath(sessionName, rootPath); err != nil {
			t.Fatalf("failed to set root path: %v", err)
		}
	}
	app.sessions = mgr
	return app
}

func TestResolveAndValidatePath(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a subdirectory and file for testing.
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		root    string
		rel     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			root:    tmpDir,
			rel:     "subdir",
			wantErr: false,
		},
		{
			name:    "valid file path",
			root:    tmpDir,
			rel:     filepath.Join("subdir", "test.txt"),
			wantErr: false,
		},
		{
			name:    "absolute path rejected",
			root:    tmpDir,
			rel:     `C:\Windows\System32`,
			wantErr: true,
			errMsg:  "path is not local",
		},
		{
			name:    "path traversal rejected",
			root:    tmpDir,
			rel:     "../../../etc/passwd",
			wantErr: true,
			errMsg:  "path is not local",
		},
		{
			name:    "nonexistent path",
			root:    tmpDir,
			rel:     "nonexistent",
			wantErr: true,
			errMsg:  "path does not exist",
		},
		{
			name:    "dot-dot path rejected",
			root:    tmpDir,
			rel:     "subdir/../../etc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveAndValidatePath(tt.root, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%s)", result)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == "" {
				t.Fatal("result should not be empty")
			}
		})
	}
}

func TestDevPanelListDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create test structure.
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "app.go"), []byte("package src"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newTestAppWithSession(t, "test-session", tmpDir)

	t.Run("root listing excludes .git and node_modules", func(t *testing.T) {
		entries, err := app.DevPanelListDir("test-session", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, entry := range entries {
			if entry.Name == ".git" || entry.Name == "node_modules" {
				t.Fatalf("excluded directory %q should not appear in listing", entry.Name)
			}
		}
		// Should contain src, main.go, README.md.
		names := make(map[string]bool)
		for _, e := range entries {
			names[e.Name] = true
		}
		if !names["src"] {
			t.Fatal("expected 'src' directory in listing")
		}
		if !names["main.go"] {
			t.Fatal("expected 'main.go' in listing")
		}
	})

	t.Run("directories listed before files", func(t *testing.T) {
		entries, err := app.DevPanelListDir("test-session", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seenFile := false
		for _, entry := range entries {
			if !entry.IsDir {
				seenFile = true
			}
			if entry.IsDir && seenFile {
				t.Fatalf("directory %q appeared after file in listing", entry.Name)
			}
		}
	})

	t.Run("subdirectory listing", func(t *testing.T) {
		entries, err := app.DevPanelListDir("test-session", "src")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 || entries[0].Name != "app.go" {
			t.Fatalf("expected [app.go], got %v", entries)
		}
	})

	t.Run("empty session name rejected", func(t *testing.T) {
		_, err := app.DevPanelListDir("", "")
		if err == nil {
			t.Fatal("expected error for empty session name")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		_, err := app.DevPanelListDir("test-session", "../../etc")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})

	t.Run("paths use forward slashes", func(t *testing.T) {
		entries, err := app.DevPanelListDir("test-session", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Path, "\\") {
				t.Fatalf("path %q contains backslash", entry.Path)
			}
		}
	})
}

func TestDevPanelReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	testContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(testContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a binary file.
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF}
	if err := os.WriteFile(filepath.Join(tmpDir, "binary.bin"), binaryContent, 0o644); err != nil {
		t.Fatal(err)
	}

	app := newTestAppWithSession(t, "test-session", tmpDir)

	t.Run("read text file", func(t *testing.T) {
		result, err := app.DevPanelReadFile("test-session", "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Content != testContent {
			t.Fatalf("content mismatch: got %q, want %q", result.Content, testContent)
		}
		if result.LineCount != 4 { // 3 lines + trailing newline counts as 4 with Count("\n")+1
			t.Fatalf("line count = %d, want 4", result.LineCount)
		}
		if result.Binary {
			t.Fatal("text file should not be detected as binary")
		}
		if result.Path != "test.txt" {
			t.Fatalf("path = %q, want %q", result.Path, "test.txt")
		}
	})

	t.Run("binary file detected", func(t *testing.T) {
		result, err := app.DevPanelReadFile("test-session", "binary.bin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Binary {
			t.Fatal("binary file should be detected as binary")
		}
		if result.Content != "" {
			t.Fatal("binary file content should be empty")
		}
	})

	t.Run("empty session name rejected", func(t *testing.T) {
		_, err := app.DevPanelReadFile("", "test.txt")
		if err == nil {
			t.Fatal("expected error for empty session name")
		}
	})

	t.Run("empty file path rejected", func(t *testing.T) {
		_, err := app.DevPanelReadFile("test-session", "")
		if err == nil {
			t.Fatal("expected error for empty file path")
		}
	})

	t.Run("directory path rejected", func(t *testing.T) {
		_, err := app.DevPanelReadFile("test-session", "subdir")
		if err == nil {
			t.Fatal("expected error for directory path")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		_, err := app.DevPanelReadFile("test-session", "../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})
}

func TestDevPanelListDirFieldCountGuard(t *testing.T) {
	// Guard: FileEntry struct field count should match frontend type definition.
	const expectedFieldCount = 4
	got := countStructFields[FileEntry]()
	if got != expectedFieldCount {
		t.Fatalf("FileEntry field count = %d, want %d; update frontend fileTreeTypes.ts", got, expectedFieldCount)
	}
}

func TestDevPanelReadFileFieldCountGuard(t *testing.T) {
	// Guard: FileContent struct field count should match frontend type definition.
	const expectedFieldCount = 6
	got := countStructFields[FileContent]()
	if got != expectedFieldCount {
		t.Fatalf("FileContent field count = %d, want %d; update frontend fileTreeTypes.ts", got, expectedFieldCount)
	}
}

func TestGitGraphCommitFieldCountGuard(t *testing.T) {
	// Guard: GitGraphCommit struct field count should match frontend type definition.
	const expectedFieldCount = 7
	got := countStructFields[GitGraphCommit]()
	if got != expectedFieldCount {
		t.Fatalf("GitGraphCommit field count = %d, want %d; update frontend gitGraphTypes.ts", got, expectedFieldCount)
	}
}

func TestGitStatusResultFieldCountGuard(t *testing.T) {
	// Guard: GitStatusResult struct field count should match frontend type definition.
	const expectedFieldCount = 6
	got := countStructFields[GitStatusResult]()
	if got != expectedFieldCount {
		t.Fatalf("GitStatusResult field count = %d, want %d; update frontend gitGraphTypes.ts", got, expectedFieldCount)
	}
}

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
			// Create the test session if it has a name we expect to find.
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

			result, err := app.resolveSessionWorkDir(tt.sessionName)
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

func TestWorkingDiffFileFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 6
	got := countStructFields[WorkingDiffFile]()
	if got != expectedFieldCount {
		t.Fatalf("WorkingDiffFile field count = %d, want %d; update frontend diffViewTypes.ts", got, expectedFieldCount)
	}
}

func TestWorkingDiffResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	got := countStructFields[WorkingDiffResult]()
	if got != expectedFieldCount {
		t.Fatalf("WorkingDiffResult field count = %d, want %d; update frontend diffViewTypes.ts", got, expectedFieldCount)
	}
}

func TestParseWorkingDiff(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFiles int
		check     func(t *testing.T, files []WorkingDiffFile)
	}{
		{
			name:      "empty string",
			input:     "",
			wantFiles: 0,
		},
		{
			name:      "whitespace only",
			input:     "   \n\n  ",
			wantFiles: 0,
		},
		{
			name: "single modified file",
			input: `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
 
+import "fmt"
 func main() {}
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "main.go" {
					t.Fatalf("path = %q, want %q", f.Path, "main.go")
				}
				if f.Status != "modified" {
					t.Fatalf("status = %q, want %q", f.Status, "modified")
				}
				if f.Additions != 1 {
					t.Fatalf("additions = %d, want 1", f.Additions)
				}
				if f.Deletions != 0 {
					t.Fatalf("deletions = %d, want 0", f.Deletions)
				}
			},
		},
		{
			name: "added file",
			input: `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+hello
+world
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "new.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "new.txt")
				}
				if f.Status != "added" {
					t.Fatalf("status = %q, want %q", f.Status, "added")
				}
				if f.Additions != 2 {
					t.Fatalf("additions = %d, want 2", f.Additions)
				}
			},
		},
		{
			name: "deleted file",
			input: `diff --git a/old.txt b/old.txt
deleted file mode 100644
index abc1234..0000000
--- a/old.txt
+++ /dev/null
@@ -1,3 +0,0 @@
-line1
-line2
-line3
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "old.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "old.txt")
				}
				if f.Status != "deleted" {
					t.Fatalf("status = %q, want %q", f.Status, "deleted")
				}
				if f.Deletions != 3 {
					t.Fatalf("deletions = %d, want 3", f.Deletions)
				}
			},
		},
		{
			name: "renamed file",
			input: `diff --git a/old_name.go b/new_name.go
similarity index 85%
rename from old_name.go
rename to new_name.go
index abc1234..def5678 100644
--- a/old_name.go
+++ b/new_name.go
@@ -1,3 +1,3 @@
 package main
 
-func oldFunc() {}
+func newFunc() {}
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "new_name.go" {
					t.Fatalf("path = %q, want %q", f.Path, "new_name.go")
				}
				if f.OldPath != "old_name.go" {
					t.Fatalf("old_path = %q, want %q", f.OldPath, "old_name.go")
				}
				if f.Status != "renamed" {
					t.Fatalf("status = %q, want %q", f.Status, "renamed")
				}
			},
		},
		{
			name: "multiple files",
			input: `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1 +1,2 @@
 package a
+// comment
diff --git a/b.go b/b.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/b.go
@@ -0,0 +1 @@
+package b
`,
			wantFiles: 2,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				if files[0].Path != "a.go" {
					t.Fatalf("first file path = %q, want %q", files[0].Path, "a.go")
				}
				if files[0].Status != "modified" {
					t.Fatalf("first file status = %q, want %q", files[0].Status, "modified")
				}
				if files[1].Path != "b.go" {
					t.Fatalf("second file path = %q, want %q", files[1].Path, "b.go")
				}
				if files[1].Status != "added" {
					t.Fatalf("second file status = %q, want %q", files[1].Status, "added")
				}
			},
		},
		{
			name: "quoted header with octal escapes",
			input: `diff --git "a/dir/\146\151\154\145\040name.txt" "b/dir/\146\151\154\145\040name.txt"
index abc1234..def5678 100644
--- "a/dir/\146\151\154\145\040name.txt"
+++ "b/dir/\146\151\154\145\040name.txt"
@@ -1 +1 @@
-old line
+new line
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "dir/file name.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "dir/file name.txt")
				}
				if f.OldPath != "dir/file name.txt" {
					t.Fatalf("old_path = %q, want %q", f.OldPath, "dir/file name.txt")
				}
				if f.Status != "modified" {
					t.Fatalf("status = %q, want %q", f.Status, "modified")
				}
				if f.Additions != 1 {
					t.Fatalf("additions = %d, want 1", f.Additions)
				}
				if f.Deletions != 1 {
					t.Fatalf("deletions = %d, want 1", f.Deletions)
				}
			},
		},
		{
			name: "unquoted path with space (same old/new)",
			input: `diff --git a/my file.txt b/my file.txt
index abc1234..def5678 100644
--- a/my file.txt
+++ b/my file.txt
@@ -1 +1,2 @@
 hello
+world
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "my file.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "my file.txt")
				}
				if f.OldPath != "my file.txt" {
					t.Fatalf("old_path = %q, want %q", f.OldPath, "my file.txt")
				}
				if f.Status != "modified" {
					t.Fatalf("status = %q, want %q", f.Status, "modified")
				}
			},
		},
		{
			name: "nested space in unquoted path",
			input: `diff --git a/dir/sub dir/file.txt b/dir/sub dir/file.txt
index abc1234..def5678 100644
--- a/dir/sub dir/file.txt
+++ b/dir/sub dir/file.txt
@@ -1 +1,2 @@
 hello
+world
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "dir/sub dir/file.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "dir/sub dir/file.txt")
				}
			},
		},
		{
			name: "rename with spaces in unquoted paths",
			input: `diff --git a/old name.txt b/new name.txt
similarity index 85%
rename from old name.txt
rename to new name.txt
index abc1234..def5678 100644
--- a/old name.txt
+++ b/new name.txt
@@ -1 +1 @@
-old content
+new content
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "new name.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "new name.txt")
				}
				if f.OldPath != "old name.txt" {
					t.Fatalf("old_path = %q, want %q", f.OldPath, "old name.txt")
				}
				if f.Status != "renamed" {
					t.Fatalf("status = %q, want %q", f.Status, "renamed")
				}
			},
		},
		{
			name: "rename where old path contains literal b/ segment",
			input: `diff --git a/old b/path.txt b/new-name.txt
similarity index 90%
rename from old b/path.txt
rename to new-name.txt
index abc1234..def5678 100644
--- a/old b/path.txt
+++ b/new-name.txt
@@ -1 +1 @@
-old content
+new content
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "new-name.txt" {
					t.Fatalf("path = %q, want %q", f.Path, "new-name.txt")
				}
				if f.OldPath != "old b/path.txt" {
					t.Fatalf("old_path = %q, want %q", f.OldPath, "old b/path.txt")
				}
				if f.Status != "renamed" {
					t.Fatalf("status = %q, want %q", f.Status, "renamed")
				}
			},
		},
		{
			name: "path containing literal b/ directory",
			input: `diff --git a/src/ b/config.go b/src/ b/config.go
index abc1234..def5678 100644
--- a/src/ b/config.go
+++ b/src/ b/config.go
@@ -1 +1,2 @@
 package config
+// updated
`,
			wantFiles: 1,
			check: func(t *testing.T, files []WorkingDiffFile) {
				t.Helper()
				f := files[0]
				if f.Path != "src/ b/config.go" {
					t.Fatalf("path = %q, want %q", f.Path, "src/ b/config.go")
				}
				if f.Status != "modified" {
					t.Fatalf("status = %q, want %q", f.Status, "modified")
				}
			},
		},
		{
			name:      "malformed input without diff header",
			input:     "not a diff at all\njust random text\n",
			wantFiles: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := parseWorkingDiff(tt.input)
			if len(files) != tt.wantFiles {
				t.Fatalf("file count = %d, want %d", len(files), tt.wantFiles)
			}
			if tt.check != nil {
				tt.check(t, files)
			}
		})
	}
}

func TestBuildUntrackedFileDiffSingle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a text file.
	textContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte(textContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a binary file.
	if err := os.WriteFile(filepath.Join(tmpDir, "bin.dat"), []byte{0x00, 0x01, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory.
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("text file produces synthetic diff", func(t *testing.T) {
		entry := buildUntrackedFileDiffSingle(tmpDir, "new.txt")
		if entry == nil {
			t.Fatal("expected non-nil entry for text file")
		}
		if entry.Path != "new.txt" {
			t.Fatalf("path = %q, want %q", entry.Path, "new.txt")
		}
		if entry.Status != "untracked" {
			t.Fatalf("status = %q, want %q", entry.Status, "untracked")
		}
		if entry.Additions != 3 {
			t.Fatalf("additions = %d, want 3", entry.Additions)
		}
		if entry.Deletions != 0 {
			t.Fatalf("deletions = %d, want 0", entry.Deletions)
		}
		if !strings.Contains(entry.Diff, "new file mode") {
			t.Fatal("diff should contain 'new file mode'")
		}
		if !strings.Contains(entry.Diff, "index 0000000..0000000") {
			t.Fatal("diff should contain synthetic index header")
		}
		if !strings.Contains(entry.Diff, "+line1") {
			t.Fatal("diff should contain '+line1'")
		}
	})

	t.Run("binary file returns nil", func(t *testing.T) {
		entry := buildUntrackedFileDiffSingle(tmpDir, "bin.dat")
		if entry != nil {
			t.Fatal("expected nil for binary file")
		}
	})

	t.Run("directory returns nil", func(t *testing.T) {
		entry := buildUntrackedFileDiffSingle(tmpDir, "subdir")
		if entry != nil {
			t.Fatal("expected nil for directory")
		}
	})

	t.Run("nonexistent file returns nil", func(t *testing.T) {
		entry := buildUntrackedFileDiffSingle(tmpDir, "missing.txt")
		if entry != nil {
			t.Fatal("expected nil for nonexistent file")
		}
	})
}

func TestParseWorkingDiff_WindowsLineEndings(t *testing.T) {
	// Windows \r\n line endings should not cause trailing \r in paths.
	input := "diff --git a/src/main.go b/src/main.go\r\nindex abc..def 100644\r\n--- a/src/main.go\r\n+++ b/src/main.go\r\n@@ -1 +1,2 @@\r\n package main\r\n+import \"fmt\"\r\n"
	files := parseWorkingDiff(input)
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if strings.ContainsRune(files[0].Path, '\r') {
		t.Fatalf("path contains \\r: %q", files[0].Path)
	}
	if files[0].Path != "src/main.go" {
		t.Fatalf("path = %q, want %q", files[0].Path, "src/main.go")
	}
}

func TestParseWorkingDiff_DiffMarkerInsideHunkNotSplit(t *testing.T) {
	input := `diff --git a/src/main.go b/src/main.go
index abc..def 100644
--- a/src/main.go
+++ b/src/main.go
@@ -1 +1,2 @@
-package main
+diff --git not-a-header
+package main
`
	files := parseWorkingDiff(input)
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Path != "src/main.go" {
		t.Fatalf("path = %q, want %q", files[0].Path, "src/main.go")
	}
	if files[0].Additions != 2 {
		t.Fatalf("additions = %d, want 2", files[0].Additions)
	}
	if files[0].Deletions != 1 {
		t.Fatalf("deletions = %d, want 1", files[0].Deletions)
	}
}

func TestParseNULSeparatedGitPaths(t *testing.T) {
	raw := []byte("plain.txt\x00dir/file with spaces.txt\x00dir/line\nbreak.txt\x00")

	paths := parseNULSeparatedGitPaths(raw)
	if len(paths) != 3 {
		t.Fatalf("path count = %d, want 3", len(paths))
	}
	if paths[0] != "plain.txt" {
		t.Fatalf("first path = %q, want %q", paths[0], "plain.txt")
	}
	if paths[1] != "dir/file with spaces.txt" {
		t.Fatalf("second path = %q, want %q", paths[1], "dir/file with spaces.txt")
	}
	if paths[2] != "dir/line\nbreak.txt" {
		t.Fatalf("third path = %q, want %q", paths[2], "dir/line\\nbreak.txt")
	}
}

func TestParseNULSeparatedGitPaths_MaxLimit(t *testing.T) {
	raw := []byte(strings.Repeat("a\x00", devPanelMaxUntrackedFilePaths+50))
	paths := parseNULSeparatedGitPaths(raw)
	if len(paths) != devPanelMaxUntrackedFilePaths {
		t.Fatalf("path count = %d, want %d", len(paths), devPanelMaxUntrackedFilePaths)
	}
}

func TestBuildUntrackedFileDiffs_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory with two text files.
	subDir := filepath.Join(tmpDir, "newdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "a.txt"), []byte("aaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("bbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := buildUntrackedFileDiffs(tmpDir, "newdir")
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}

	paths := make(map[string]bool)
	for _, e := range entries {
		paths[e.Path] = true
		if !strings.HasPrefix(e.Path, "newdir/") {
			t.Fatalf("path %q should start with 'newdir/'", e.Path)
		}
		if e.Status != "untracked" {
			t.Fatalf("status = %q, want 'untracked'", e.Status)
		}
		if strings.Contains(e.Path, "\\") {
			t.Fatalf("path %q contains backslash", e.Path)
		}
	}
	if !paths["newdir/a.txt"] {
		t.Fatal("expected entry for newdir/a.txt")
	}
	if !paths["newdir/b.txt"] {
		t.Fatal("expected entry for newdir/b.txt")
	}
}

func TestBuildUntrackedFileDiffs_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "single.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := buildUntrackedFileDiffs(tmpDir, "single.txt")
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].Path != "single.txt" {
		t.Fatalf("path = %q, want %q", entries[0].Path, "single.txt")
	}
	if entries[0].Status != "untracked" {
		t.Fatalf("status = %q, want 'untracked'", entries[0].Status)
	}
}

func TestBuildUntrackedFileDiffs_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directories: parent/child/file.txt
	nested := filepath.Join(tmpDir, "parent", "child")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("deep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := buildUntrackedFileDiffs(tmpDir, "parent")
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].Path != "parent/child/file.txt" {
		t.Fatalf("path = %q, want %q", entries[0].Path, "parent/child/file.txt")
	}
}

func TestBuildUntrackedFileDiffs_SkipsBinary(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "mixed")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Text file.
	if err := os.WriteFile(filepath.Join(subDir, "text.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Binary file.
	if err := os.WriteFile(filepath.Join(subDir, "bin.dat"), []byte{0x00, 0x01, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}

	entries := buildUntrackedFileDiffs(tmpDir, "mixed")
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1 (binary should be skipped)", len(entries))
	}
	if entries[0].Path != "mixed/text.txt" {
		t.Fatalf("path = %q, want %q", entries[0].Path, "mixed/text.txt")
	}
}

func TestBuildUntrackedFileDiffs_ExcludedDirsSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg.js"), []byte("pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package src\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := buildUntrackedFileDiffs(tmpDir, "project")
	// node_modules should be skipped.
	for _, e := range entries {
		if strings.Contains(e.Path, "node_modules") {
			t.Fatalf("node_modules should be excluded but found: %s", e.Path)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].Path != "project/src/app.go" {
		t.Fatalf("path = %q, want %q", entries[0].Path, "project/src/app.go")
	}
}

func TestBuildUntrackedFileDiffSingle_OldPathEmpty(t *testing.T) {
	// SUG-07: Untracked (new) files must have empty OldPath.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := buildUntrackedFileDiffSingle(tmpDir, "new.txt")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.OldPath != "" {
		t.Fatalf("OldPath = %q, want empty string for untracked file", entry.OldPath)
	}
}

func TestBuildUntrackedFileDiffSingle_EmptyFile(t *testing.T) {
	// SUG-06: Empty files produce diff header only, no hunk.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "empty.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	entry := buildUntrackedFileDiffSingle(tmpDir, "empty.txt")
	if entry == nil {
		t.Fatal("expected non-nil entry for empty file")
	}
	if entry.Additions != 0 {
		t.Fatalf("additions = %d, want 0 for empty file", entry.Additions)
	}
	if strings.Contains(entry.Diff, "@@") {
		t.Fatalf("empty file diff should not contain hunk header, got:\n%s", entry.Diff)
	}
	if !strings.Contains(entry.Diff, "new file mode") {
		t.Fatal("empty file diff should contain 'new file mode'")
	}
}

func TestBuildUntrackedFileDiffSingle_PathEscape(t *testing.T) {
	// IMP-03: Paths that escape workDir must return nil.
	tmpDir := t.TempDir()

	entry := buildUntrackedFileDiffSingle(tmpDir, "../../../etc/passwd")
	if entry != nil {
		t.Fatal("expected nil for path that escapes workDir")
	}
}

func TestBuildUntrackedFileDiffSingle_SymlinkSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(tmpDir, "escape-link")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink creation not available in this environment: %v", err)
	}

	entry := buildUntrackedFileDiffSingle(tmpDir, "escape-link")
	if entry != nil {
		t.Fatal("expected nil for symlinked untracked file")
	}
}

func TestBuildUntrackedFileDiffSingle_SpaceInPath(t *testing.T) {
	// IMP-06: Paths with spaces should produce quoted diff headers.
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "my dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := buildUntrackedFileDiffSingle(tmpDir, "my dir/file.txt")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	// The diff header should contain quoted path.
	if !strings.Contains(entry.Diff, `"my dir/file.txt"`) {
		t.Fatalf("expected quoted path in diff header, got:\n%s", entry.Diff)
	}
	// But the Path field itself should remain unquoted for frontend use.
	if entry.Path != "my dir/file.txt" {
		t.Fatalf("Path = %q, want %q", entry.Path, "my dir/file.txt")
	}
}

func TestBuildUntrackedFileDiffSingle_CachedFileInfo(t *testing.T) {
	// SUG-05: Passing cached FileInfo avoids redundant os.Stat.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "cached.txt"), []byte("data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(tmpDir, "cached.txt"))
	if err != nil {
		t.Fatal(err)
	}

	entry := buildUntrackedFileDiffSingle(tmpDir, "cached.txt", info)
	if entry == nil {
		t.Fatal("expected non-nil entry with cached FileInfo")
	}
	if entry.Path != "cached.txt" {
		t.Fatalf("Path = %q, want %q", entry.Path, "cached.txt")
	}
}

func TestBuildUntrackedFileDiffSingle_SizeGuardDuringRead(t *testing.T) {
	// Defensive guard: even with stale cached metadata, reads must never exceed
	// devPanelMaxUntrackedFileSize.
	tmpDir := t.TempDir()
	large := strings.Repeat("x", int(devPanelMaxUntrackedFileSize)+32)
	if err := os.WriteFile(filepath.Join(tmpDir, "growing.txt"), []byte(large), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate stale metadata from an earlier walk/stat snapshot.
	staleInfo := fakeFileInfo{
		name: "growing.txt",
		size: 1,
		mode: 0,
	}
	entry := buildUntrackedFileDiffSingle(tmpDir, "growing.txt", staleInfo)
	if entry != nil {
		t.Fatalf("expected nil for oversized file despite stale cached size, got entry for %q", entry.Path)
	}
}

func TestNeedsGitQuoting(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"simple.txt", false},
		{"dir/file.go", false},
		{"my file.txt", true},
		{"dir/sub dir/file.txt", true},
		{`path\with\backslash`, true},
		{"path\"with\"quote", true},
		{"path\twith\ttab", true},
		{"日本語.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := needsGitQuoting(tt.path)
			if got != tt.want {
				t.Fatalf("needsGitQuoting(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsFreshRepoStatusOutput(t *testing.T) {
	tests := []struct {
		name   string
		output []byte
		want   bool
	}{
		{
			name:   "no commits yet status is treated as fresh repo",
			output: []byte("## No commits yet on main\n?? new.txt\n"),
			want:   true,
		},
		{
			name:   "legacy initial commit status is treated as fresh repo",
			output: []byte("## Initial commit on master\r\nA  new.txt\r\n"),
			want:   true,
		},
		{
			name:   "regular branch status is not fresh repo",
			output: []byte("## main...origin/main\n"),
			want:   false,
		},
		{
			name:   "empty output is not fresh repo",
			output: []byte(""),
			want:   false,
		},
		{
			name:   "output without branch header is not fresh repo",
			output: []byte("?? a.txt\n"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFreshRepoStatusOutput(tt.output)
			if got != tt.want {
				t.Fatalf("isFreshRepoStatusOutput(%q) = %v, want %v", string(tt.output), got, tt.want)
			}
		})
	}
}

func TestDetectFreshRepoState(t *testing.T) {
	headErr := errors.New("head probe failed")
	statusErr := errors.New("status probe failed")

	tests := []struct {
		name          string
		runGit        func(string, []string) ([]byte, error)
		wantFresh     bool
		wantErr       bool
		wantErrSubstr string
		wantHeadErr   bool
		wantStatusErr bool
	}{
		{
			name: "head exists returns non-fresh",
			runGit: func(_ string, args []string) ([]byte, error) {
				if strings.Join(args, " ") == "rev-parse --verify --quiet HEAD" {
					return []byte("abc123\n"), nil
				}
				t.Fatalf("unexpected git args: %v", args)
				return nil, nil
			},
			wantFresh: false,
		},
		{
			name: "head missing with no commits status returns fresh",
			runGit: func(_ string, args []string) ([]byte, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify --quiet HEAD":
					return nil, headErr
				case "status --porcelain --branch":
					return []byte("## No commits yet on main\n?? new.txt\n"), nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return nil, nil
				}
			},
			wantFresh: true,
		},
		{
			name: "head missing with non-fresh status returns error",
			runGit: func(_ string, args []string) ([]byte, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify --quiet HEAD":
					return nil, headErr
				case "status --porcelain --branch":
					return []byte("## main...origin/main\n"), nil
				default:
					t.Fatalf("unexpected git args: %v", args)
					return nil, nil
				}
			},
			wantErr:       true,
			wantErrSubstr: "failed to verify HEAD commit",
			wantHeadErr:   true,
		},
		{
			name: "head and status probe failure returns joined error",
			runGit: func(_ string, args []string) ([]byte, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify --quiet HEAD":
					return nil, headErr
				case "status --porcelain --branch":
					return nil, statusErr
				default:
					t.Fatalf("unexpected git args: %v", args)
					return nil, nil
				}
			},
			wantErr:       true,
			wantErrSubstr: "failed to verify HEAD state",
			wantHeadErr:   true,
			wantStatusErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fresh, err := detectFreshRepoState("ignored", tt.runGit)
			if fresh != tt.wantFresh {
				t.Fatalf("fresh = %v, want %v (err=%v)", fresh, tt.wantFresh, err)
			}
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErrSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr)) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErrSubstr)
			}
			if tt.wantHeadErr && !errors.Is(err, headErr) {
				t.Fatalf("error %q does not wrap headErr", err)
			}
			if tt.wantStatusErr && !errors.Is(err, statusErr) {
				t.Fatalf("error %q does not wrap statusErr", err)
			}
		})
	}
}

func TestDetectFreshRepoState_NilRunner(t *testing.T) {
	_, err := detectFreshRepoState("ignored", nil)
	if err == nil {
		t.Fatal("expected error for nil git runner")
	}
	if !strings.Contains(err.Error(), "git runner is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fakeFileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

// countStructFields returns the number of fields in a struct type.
func countStructFields[T any]() int {
	var zero T
	return reflect.TypeOf(zero).NumField()
}
