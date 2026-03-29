package devpanel

import (
	"errors"
	"os"
	execStdlib "os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testIsPathWithinBase is a simple implementation for testing.
func testIsPathWithinBase(path, base string) bool {
	relPath, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// newTestService creates a Service with a ResolveSessionDir that returns rootPath
// for the given sessionName.
func newTestService(sessionName, rootPath string) *Service {
	return NewService(Deps{
		ResolveSessionDir: func(name string, _ bool) (string, error) {
			if name == sessionName {
				if rootPath == "" {
					return "", errors.New("session has no root path")
				}
				return rootPath, nil
			}
			return "", errors.New("session not found")
		},
		IsPathWithinBase: testIsPathWithinBase,
	})
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

	svc := newTestService("test", tmpDir)

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
			result, err := svc.ResolveAndValidatePath(tt.root, tt.rel)
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

func TestListDir(t *testing.T) {
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

	svc := newTestService("test-session", tmpDir)

	t.Run("root listing excludes .git and node_modules", func(t *testing.T) {
		entries, err := svc.ListDir("test-session", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, entry := range entries {
			if entry.Name == ".git" || entry.Name == "node_modules" {
				t.Fatalf("excluded directory %q should not appear in listing", entry.Name)
			}
		}
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
		entries, err := svc.ListDir("test-session", "")
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
		entries, err := svc.ListDir("test-session", "src")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 || entries[0].Name != "app.go" {
			t.Fatalf("expected [app.go], got %v", entries)
		}
	})

	t.Run("empty session name rejected", func(t *testing.T) {
		_, err := svc.ListDir("", "")
		if err == nil {
			t.Fatal("expected error for empty session name")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		_, err := svc.ListDir("test-session", "../../etc")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})

	t.Run("paths use forward slashes", func(t *testing.T) {
		entries, err := svc.ListDir("test-session", "")
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

func TestReadFile(t *testing.T) {
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

	svc := newTestService("test-session", tmpDir)

	t.Run("read text file", func(t *testing.T) {
		result, err := svc.ReadFile("test-session", "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Content != testContent {
			t.Fatalf("content mismatch: got %q, want %q", result.Content, testContent)
		}
		if result.LineCount != 4 {
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
		result, err := svc.ReadFile("test-session", "binary.bin")
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
		_, err := svc.ReadFile("", "test.txt")
		if err == nil {
			t.Fatal("expected error for empty session name")
		}
	})

	t.Run("empty file path rejected", func(t *testing.T) {
		_, err := svc.ReadFile("test-session", "")
		if err == nil {
			t.Fatal("expected error for empty file path")
		}
	})

	t.Run("directory path rejected", func(t *testing.T) {
		_, err := svc.ReadFile("test-session", "subdir")
		if err == nil {
			t.Fatal("expected error for directory path")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		_, err := svc.ReadFile("test-session", "../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path traversal")
		}
	})
}

// ---------------------------------------------------------------------------
// Struct field count guards
// ---------------------------------------------------------------------------

func TestFileEntryFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	got := reflect.TypeFor[FileEntry]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("FileEntry field count = %d, want %d; update frontend fileTreeTypes.ts", got, expectedFieldCount)
	}
}

func TestFileContentFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 6
	got := reflect.TypeFor[FileContent]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("FileContent field count = %d, want %d; update frontend fileTreeTypes.ts", got, expectedFieldCount)
	}
}

func TestGitGraphCommitFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 7
	got := reflect.TypeFor[GitGraphCommit]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("GitGraphCommit field count = %d, want %d; update frontend gitGraphTypes.ts", got, expectedFieldCount)
	}
}

func TestGitStatusResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 8 // branch, modified, staged, untracked, conflicted, ahead, behind, upstream_configured
	got := reflect.TypeFor[GitStatusResult]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("GitStatusResult field count = %d, want %d; update frontend models.ts", got, expectedFieldCount)
	}
}

func TestWorkingDiffFileFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 6
	got := reflect.TypeFor[WorkingDiffFile]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("WorkingDiffFile field count = %d, want %d; update frontend diffViewTypes.ts", got, expectedFieldCount)
	}
}

func TestWorkingDiffResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	got := reflect.TypeFor[WorkingDiffResult]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("WorkingDiffResult field count = %d, want %d; update frontend diffViewTypes.ts", got, expectedFieldCount)
	}
}

func TestCommitResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 2
	got := reflect.TypeFor[CommitResult]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("CommitResult field count = %d, want %d; update frontend model", got, expectedFieldCount)
	}
}

func TestPushResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 3
	got := reflect.TypeFor[PushResult]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("PushResult field count = %d, want %d; update frontend model", got, expectedFieldCount)
	}
}

func TestPullResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 2
	got := reflect.TypeFor[PullResult]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("PullResult field count = %d, want %d; update frontend model", got, expectedFieldCount)
	}
}

// ---------------------------------------------------------------------------
// Diff parsing tests
// ---------------------------------------------------------------------------

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
				if files[0].Path != "a.go" || files[0].Status != "modified" {
					t.Fatalf("first file: path=%q status=%q", files[0].Path, files[0].Status)
				}
				if files[1].Path != "b.go" || files[1].Status != "added" {
					t.Fatalf("second file: path=%q status=%q", files[1].Path, files[1].Status)
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
			},
		},
		{
			name: "unquoted path with space",
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
				if files[0].Path != "my file.txt" {
					t.Fatalf("path = %q, want %q", files[0].Path, "my file.txt")
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
				if files[0].Path != "dir/sub dir/file.txt" {
					t.Fatalf("path = %q, want %q", files[0].Path, "dir/sub dir/file.txt")
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

func TestParseWorkingDiff_WindowsLineEndings(t *testing.T) {
	input := "diff --git a/src/main.go b/src/main.go\r\nindex abc..def 100644\r\n--- a/src/main.go\r\n+++ b/src/main.go\r\n@@ -1 +1,2 @@\r\n package main\r\n+import \"fmt\"\r\n"
	files := parseWorkingDiff(input)
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if strings.ContainsRune(files[0].Path, '\r') {
		t.Fatalf("path contains \\r: %q", files[0].Path)
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
	if files[0].Additions != 2 {
		t.Fatalf("additions = %d, want 2", files[0].Additions)
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
}

func TestParseNULSeparatedGitPaths_MaxLimit(t *testing.T) {
	raw := []byte(strings.Repeat("a\x00", maxUntrackedFilePaths+50))
	paths := parseNULSeparatedGitPaths(raw)
	if len(paths) != maxUntrackedFilePaths {
		t.Fatalf("path count = %d, want %d", len(paths), maxUntrackedFilePaths)
	}
}

func TestBuildUntrackedFileDiffSingle(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "bin.dat"), []byte{0x00, 0x01, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)

	t.Run("text file produces synthetic diff", func(t *testing.T) {
		entry := svc.buildUntrackedFileDiffSingle(tmpDir, "new.txt")
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
		if !strings.Contains(entry.Diff, "new file mode") {
			t.Fatal("diff should contain 'new file mode'")
		}
	})

	t.Run("binary file returns nil", func(t *testing.T) {
		entry := svc.buildUntrackedFileDiffSingle(tmpDir, "bin.dat")
		if entry != nil {
			t.Fatal("expected nil for binary file")
		}
	})

	t.Run("directory returns nil", func(t *testing.T) {
		entry := svc.buildUntrackedFileDiffSingle(tmpDir, "subdir")
		if entry != nil {
			t.Fatal("expected nil for directory")
		}
	})

	t.Run("nonexistent file returns nil", func(t *testing.T) {
		entry := svc.buildUntrackedFileDiffSingle(tmpDir, "missing.txt")
		if entry != nil {
			t.Fatal("expected nil for nonexistent file")
		}
	})
}

func TestBuildUntrackedFileDiffs_Directory(t *testing.T) {
	tmpDir := t.TempDir()
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

	svc := newTestService("test", tmpDir)
	entries := svc.buildUntrackedFileDiffs(tmpDir, "newdir")
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	paths := make(map[string]bool)
	for _, e := range entries {
		paths[e.Path] = true
		if !strings.HasPrefix(e.Path, "newdir/") {
			t.Fatalf("path %q should start with 'newdir/'", e.Path)
		}
	}
	if !paths["newdir/a.txt"] || !paths["newdir/b.txt"] {
		t.Fatal("expected entries for newdir/a.txt and newdir/b.txt")
	}
}

func TestBuildUntrackedFileDiffs_SkipsBinary(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "mixed")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "text.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "bin.dat"), []byte{0x00, 0x01, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entries := svc.buildUntrackedFileDiffs(tmpDir, "mixed")
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1 (binary should be skipped)", len(entries))
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
	if err := os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entries := svc.buildUntrackedFileDiffs(tmpDir, "project")
	for _, e := range entries {
		if strings.Contains(e.Path, "node_modules") {
			t.Fatalf("excluded directory content %q should not appear", e.Path)
		}
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
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
			name:   "no commits yet",
			output: []byte("## No commits yet on main\n?? new.txt\n"),
			want:   true,
		},
		{
			name:   "legacy initial commit",
			output: []byte("## Initial commit on master\r\nA  new.txt\r\n"),
			want:   true,
		},
		{
			name:   "regular branch",
			output: []byte("## main...origin/main\n"),
			want:   false,
		},
		{
			name:   "empty output",
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
			name: "head missing with no commits returns fresh",
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
	if err == nil || !strings.Contains(err.Error(), "git runner is required") {
		t.Fatalf("expected 'git runner is required' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Git operations validation tests
// ---------------------------------------------------------------------------

func TestValidateGitFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{name: "valid relative path", path: "src/main.go", wantErr: false},
		{name: "valid simple filename", path: "file.txt", wantErr: false},
		{name: "valid nested path", path: "internal/git/command.go", wantErr: false},
		{name: "empty path rejected", path: "", wantErr: true, errMsg: "file path is required"},
		{name: "whitespace only rejected", path: "   ", wantErr: true, errMsg: "file path is required"},
		{name: "absolute path rejected", path: `C:\Windows\System32\cmd.exe`, wantErr: true, errMsg: "file path must be relative"},
		{name: "path traversal rejected", path: "../../../etc/passwd", wantErr: true, errMsg: "file path is not local"},
		{name: "dot-dot in middle rejected", path: "src/../../secret.txt", wantErr: true, errMsg: "file path is not local"},
		{name: "leading spaces trimmed", path: "  src/main.go  ", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitFilePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"first\nsecond\nthird", "first"},
		{"", ""},
		{"\n", ""},
		{"msg\n", "msg"},
	}
	for _, tt := range tests {
		got := firstLine(tt.input)
		if got != tt.want {
			t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDecodeGitPathLiteralForStatusPaths(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{name: "plain ASCII", raw: "src/main.go", want: "src/main.go", wantOK: true},
		{name: "quoted with spaces", raw: strconv.Quote("plans/plans - copy/test.go"), want: "plans/plans - copy/test.go", wantOK: true},
		{name: "quoted with Japanese", raw: strconv.Quote("plans/コピー/test.go"), want: "plans/コピー/test.go", wantOK: true},
		{name: "quoted deep nested path with spaces and Japanese", raw: strconv.Quote("plans/plans - コピー/plans - コピー/新規 テキスト ドキュメント.txt"), want: "plans/plans - コピー/plans - コピー/新規 テキスト ドキュメント.txt", wantOK: true},
		{name: "empty string", raw: "", want: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeGitPathLiteral(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Untracked file diff edge-case tests (IMP-1: migrated from app_devpanel_api_test.go)
// ---------------------------------------------------------------------------

func TestBuildUntrackedFileDiffSingle_PathEscape(t *testing.T) {
	tmpDir := t.TempDir()
	svc := newTestService("test", tmpDir)

	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "../../../etc/passwd")
	if entry != nil {
		t.Fatal("expected nil for path that escapes workDir")
	}
}

func TestBuildUntrackedFileDiffSingle_OldPathEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "new.txt")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.OldPath != "" {
		t.Fatalf("OldPath = %q, want empty string for untracked file", entry.OldPath)
	}
}

func TestBuildUntrackedFileDiffSingle_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "empty.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "empty.txt")
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

func TestBuildUntrackedFileDiffSingle_SizeGuardDuringRead(t *testing.T) {
	tmpDir := t.TempDir()
	large := strings.Repeat("x", int(maxUntrackedFileSize)+32)
	if err := os.WriteFile(filepath.Join(tmpDir, "growing.txt"), []byte(large), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)

	// Simulate stale metadata from an earlier walk/stat snapshot.
	staleInfo := fakeFileInfo{
		name: "growing.txt",
		size: 1,
		mode: 0,
	}
	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "growing.txt", staleInfo)
	if entry != nil {
		t.Fatalf("expected nil for oversized file despite stale cached size, got entry for %q", entry.Path)
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

	svc := newTestService("test", tmpDir)
	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "escape-link")
	if entry != nil {
		t.Fatal("expected nil for symlinked untracked file")
	}
}

func TestBuildUntrackedFileDiffSingle_SpaceInPath(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "my dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "my dir/file.txt")
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
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "cached.txt"), []byte("data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(tmpDir, "cached.txt"))
	if err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entry := svc.buildUntrackedFileDiffSingle(tmpDir, "cached.txt", info)
	if entry == nil {
		t.Fatal("expected non-nil entry with cached FileInfo")
	}
	if entry.Path != "cached.txt" {
		t.Fatalf("Path = %q, want %q", entry.Path, "cached.txt")
	}
}

func TestBuildUntrackedFileDiffs_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "single.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entries := svc.buildUntrackedFileDiffs(tmpDir, "single.txt")
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

	nested := filepath.Join(tmpDir, "parent", "child")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("deep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	entries := svc.buildUntrackedFileDiffs(tmpDir, "parent")
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].Path != "parent/child/file.txt" {
		t.Fatalf("path = %q, want %q", entries[0].Path, "parent/child/file.txt")
	}
}

// ---------------------------------------------------------------------------
// Git session validation tests (IMP-1: migrated from app_devpanel_git_ops_api_test.go)
// ---------------------------------------------------------------------------

func TestResolveAndValidateGitSession(t *testing.T) {
	t.Run("empty session name returns error", func(t *testing.T) {
		svc := newTestService("test", t.TempDir())
		_, err := svc.resolveAndValidateGitSession("")
		if err == nil {
			t.Fatal("expected error for empty session name")
		}
		if !strings.Contains(err.Error(), "session name is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("whitespace session name returns error", func(t *testing.T) {
		svc := newTestService("test", t.TempDir())
		_, err := svc.resolveAndValidateGitSession("   ")
		if err == nil {
			t.Fatal("expected error for whitespace session name")
		}
		if !strings.Contains(err.Error(), "session name is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-existent session returns error", func(t *testing.T) {
		svc := newTestService("test-session", t.TempDir())
		_, err := svc.resolveAndValidateGitSession("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent session")
		}
	})

	t.Run("non-git directory returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		svc := newTestService("test-session", tmpDir)
		_, err := svc.resolveAndValidateGitSession("test-session")
		if err == nil {
			t.Fatal("expected error for non-git directory")
		}
		if !strings.Contains(err.Error(), "not a git repository") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewService_PanicsOnNilDeps(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil deps, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "required function fields must be non-nil") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	NewService(Deps{})
}

func TestNewService_PanicsOnPartialDeps(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for partial deps, got none")
		}
	}()
	NewService(Deps{
		ResolveSessionDir: func(_ string, _ bool) (string, error) { return "", nil },
		// IsPathWithinBase is nil — should panic
	})
}

// ---------------------------------------------------------------------------
// GitStatus tests — conflict detection and field initialization
// ---------------------------------------------------------------------------

// initGitRepo creates a git repo with an initial commit in tmpDir and returns
// the default branch name (varies by git config: "main" or "master").
func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := gitCmd(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("initGitRepo %v failed: %v\n%s", args, err, out)
		}
	}
	// Detect the default branch name.
	cmd := gitCmd("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to detect default branch: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitCmd(name string, args ...string) *execStdlib.Cmd {
	return execStdlib.Command(name, args...)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := gitCmd("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestGitStatus_ConflictDetection(t *testing.T) {
	tmpDir := t.TempDir()
	defaultBranch := initGitRepo(t, tmpDir)

	// Create a file on default branch and commit.
	if err := os.WriteFile(filepath.Join(tmpDir, "conflict.txt"), []byte("main content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmpDir, "add", "conflict.txt")
	gitRun(t, tmpDir, "commit", "-m", "add conflict.txt on main")

	// Create feature branch, modify the same file differently.
	gitRun(t, tmpDir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(tmpDir, "conflict.txt"), []byte("feature content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmpDir, "add", "conflict.txt")
	gitRun(t, tmpDir, "commit", "-m", "modify conflict.txt on feature")

	// Go back to default branch, modify the same file differently.
	gitRun(t, tmpDir, "checkout", defaultBranch)
	if err := os.WriteFile(filepath.Join(tmpDir, "conflict.txt"), []byte("main changed content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmpDir, "add", "conflict.txt")
	gitRun(t, tmpDir, "commit", "-m", "modify conflict.txt on main")

	// Merge feature into main — this will create a conflict.
	cmd := gitCmd("git", "merge", "feature")
	cmd.Dir = tmpDir
	// Merge is expected to fail with a conflict.
	_ = cmd.Run()

	svc := newTestService("test", tmpDir)
	result, err := svc.GitStatus("test")
	if err != nil {
		t.Fatalf("GitStatus failed: %v", err)
	}

	// Verify conflict is detected.
	if len(result.Conflicted) == 0 {
		t.Fatal("expected at least one conflicted file")
	}
	found := slices.Contains(result.Conflicted, "conflict.txt")
	if !found {
		t.Fatalf("expected conflict.txt in Conflicted list, got %v", result.Conflicted)
	}

	// C-1 fix: Conflicted files must NOT appear in Staged or Modified.
	for _, f := range result.Staged {
		if f == "conflict.txt" {
			t.Fatal("conflicted file should NOT appear in Staged list (double-counting bug)")
		}
	}
	for _, f := range result.Modified {
		if f == "conflict.txt" {
			t.Fatal("conflicted file should NOT appear in Modified list (double-counting bug)")
		}
	}
}

func TestGitStatus_EmptySliceInitialization(t *testing.T) {
	tmpDir := t.TempDir()
	_ = initGitRepo(t, tmpDir)

	svc := newTestService("test", tmpDir)
	result, err := svc.GitStatus("test")
	if err != nil {
		t.Fatalf("GitStatus failed: %v", err)
	}

	// S-5: All slice fields must be non-nil (empty, not null in JSON).
	if result.Modified == nil {
		t.Fatal("Modified should be empty slice, not nil")
	}
	if result.Staged == nil {
		t.Fatal("Staged should be empty slice, not nil")
	}
	if result.Untracked == nil {
		t.Fatal("Untracked should be empty slice, not nil")
	}
	if result.Conflicted == nil {
		t.Fatal("Conflicted should be empty slice, not nil")
	}

	// Repo without upstream should report UpstreamConfigured=false.
	if result.UpstreamConfigured {
		t.Fatal("expected UpstreamConfigured=false for repo without upstream")
	}
}

func TestGitStatus_StagedAndModifiedClassification(t *testing.T) {
	tmpDir := t.TempDir()
	_ = initGitRepo(t, tmpDir)

	// Create a file, stage it, then modify it again.
	if err := os.WriteFile(filepath.Join(tmpDir, "dual.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmpDir, "add", "dual.txt")
	gitRun(t, tmpDir, "commit", "-m", "add dual.txt")

	// Modify and stage.
	if err := os.WriteFile(filepath.Join(tmpDir, "dual.txt"), []byte("staged change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmpDir, "add", "dual.txt")
	// Modify again (unstaged).
	if err := os.WriteFile(filepath.Join(tmpDir, "dual.txt"), []byte("unstaged change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	result, err := svc.GitStatus("test")
	if err != nil {
		t.Fatalf("GitStatus failed: %v", err)
	}

	// dual.txt should be in both Staged and Modified.
	foundStaged := slices.Contains(result.Staged, "dual.txt")
	foundModified := slices.Contains(result.Modified, "dual.txt")
	if !foundStaged {
		t.Fatal("dual.txt should be in Staged")
	}
	if !foundModified {
		t.Fatal("dual.txt should be in Modified")
	}
	// Should NOT be in Conflicted.
	for _, f := range result.Conflicted {
		if f == "dual.txt" {
			t.Fatal("non-conflicted file should not appear in Conflicted")
		}
	}
}

// ---------------------------------------------------------------------------
// GitPush / GitPull / UpstreamConfigured tests
// ---------------------------------------------------------------------------

func TestGitPush_WithUpstream(t *testing.T) {
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	// Create a file and push so upstream tracking is established.
	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial commit")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Make another change to push.
	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "second commit")

	svc := newTestService("test", cloneDir)
	result, err := svc.GitPush("test")
	if err != nil {
		t.Fatalf("GitPush failed: %v", err)
	}
	if result.RemoteName != "origin" {
		t.Fatalf("RemoteName = %q, want %q", result.RemoteName, "origin")
	}
	if result.UpstreamSet {
		t.Fatal("UpstreamSet should be false for existing upstream")
	}
}

func TestGitPush_WithoutUpstream(t *testing.T) {
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	// Initial commit on default branch.
	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial commit")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Create a new branch without upstream.
	gitRun(t, cloneDir, "checkout", "-b", "feature/no-upstream")
	if err := os.WriteFile(filepath.Join(cloneDir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "feature commit")

	svc := newTestService("test", cloneDir)
	result, err := svc.GitPush("test")
	if err != nil {
		t.Fatalf("GitPush failed: %v", err)
	}
	if !result.UpstreamSet {
		t.Fatal("UpstreamSet should be true when upstream was not configured")
	}
	if result.BranchName != "feature/no-upstream" {
		t.Fatalf("BranchName = %q, want %q", result.BranchName, "feature/no-upstream")
	}
}

func TestGitPush_EmptySession(t *testing.T) {
	svc := newTestService("test", t.TempDir())
	_, err := svc.GitPush("")
	if err == nil {
		t.Fatal("expected error for empty session name")
	}
}

func TestGitPull_FastForward(t *testing.T) {
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Create a second clone and push a commit from there.
	clone2Dir := t.TempDir()
	gitRun(t, clone2Dir, "clone", bareDir, ".")
	gitRun(t, clone2Dir, "config", "user.email", "test@test.com")
	gitRun(t, clone2Dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone2Dir, "file.txt"), []byte("updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, clone2Dir, "add", ".")
	gitRun(t, clone2Dir, "commit", "-m", "update from clone2")
	gitRun(t, clone2Dir, "push")

	// Pull in the first clone.
	svc := newTestService("test", cloneDir)
	result, err := svc.GitPull("test")
	if err != nil {
		t.Fatalf("GitPull failed: %v", err)
	}
	if !result.Updated {
		t.Fatal("expected Updated=true after pulling new commits")
	}
}

func TestGitPull_AlreadyUpToDate(t *testing.T) {
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	svc := newTestService("test", cloneDir)
	result, err := svc.GitPull("test")
	if err != nil {
		t.Fatalf("GitPull failed: %v", err)
	}
	if result.Updated {
		t.Fatal("expected Updated=false when already up to date")
	}
}

func TestGitPull_NonFastForward(t *testing.T) {
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Push a different commit from a second clone.
	clone2Dir := t.TempDir()
	gitRun(t, clone2Dir, "clone", bareDir, ".")
	gitRun(t, clone2Dir, "config", "user.email", "test@test.com")
	gitRun(t, clone2Dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone2Dir, "file.txt"), []byte("clone2 change"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, clone2Dir, "add", ".")
	gitRun(t, clone2Dir, "commit", "-m", "clone2 commit")
	gitRun(t, clone2Dir, "push")

	// Make a local commit in the first clone that diverges.
	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("local change"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "local divergent commit")

	// Fetch so @{u} is updated but branches have diverged.
	gitRun(t, cloneDir, "fetch")

	svc := newTestService("test", cloneDir)
	_, err := svc.GitPull("test")
	if err == nil {
		t.Fatal("expected error for non-fast-forward pull")
	}
	if !strings.Contains(err.Error(), "fast-forward is not possible") {
		t.Fatalf("expected non-fast-forward error, got: %v", err)
	}
}

func TestGitStatus_UpstreamConfigured(t *testing.T) {
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	svc := newTestService("test", cloneDir)
	result, err := svc.GitStatus("test")
	if err != nil {
		t.Fatalf("GitStatus failed: %v", err)
	}
	if !result.UpstreamConfigured {
		t.Fatal("expected UpstreamConfigured=true for cloned repo with upstream")
	}
}

func TestGitStatus_DetachedHeadBranch(t *testing.T) {
	tmpDir := t.TempDir()
	_ = initGitRepo(t, tmpDir)

	// Detach HEAD.
	gitRun(t, tmpDir, "checkout", "--detach")

	svc := newTestService("test", tmpDir)
	result, err := svc.GitStatus("test")
	if err != nil {
		t.Fatalf("GitStatus failed: %v", err)
	}
	if result.Branch != DetachedHEADSentinel {
		t.Fatalf("Branch = %q, want %q", result.Branch, DetachedHEADSentinel)
	}
}

func TestIsNonFastForwardError(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		wantFF  bool
		pathDoc string // which detection path this exercises
	}{
		{
			name:    "english message: not possible to fast-forward",
			errMsg:  "fatal: Not possible to fast-forward, aborting.",
			wantFF:  true,
			pathDoc: "primary/english-match",
		},
		{
			name:    "english message: cannot fast-forward",
			errMsg:  "error: Cannot fast-forward your working tree",
			wantFF:  true,
			pathDoc: "primary/english-match",
		},
		{
			name:    "unrelated error is not non-fast-forward",
			errMsg:  "fatal: remote origin already exists",
			wantFF:  false,
			pathDoc: "no-match",
		},
		{
			name:    "empty error",
			errMsg:  "",
			wantFF:  false,
			pathDoc: "no-match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonFastForwardError("", errors.New(tt.errMsg))
			if got != tt.wantFF {
				t.Errorf("isNonFastForwardError(%q) = %v, want %v [path: %s]", tt.errMsg, got, tt.wantFF, tt.pathDoc)
			}
		})
	}
}

func TestIsNonFastForwardError_FallbackExitCode(t *testing.T) {
	// The locale-independent fallback path uses exec.ExitError with exit code 1.
	// This test requires a real git repo with diverged branches to exercise
	// the merge-base --is-ancestor probe.
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Push a different commit from a second clone to create divergence.
	clone2Dir := t.TempDir()
	gitRun(t, clone2Dir, "clone", bareDir, ".")
	gitRun(t, clone2Dir, "config", "user.email", "test@test.com")
	gitRun(t, clone2Dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone2Dir, "file.txt"), []byte("clone2"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, clone2Dir, "add", ".")
	gitRun(t, clone2Dir, "commit", "-m", "clone2")
	gitRun(t, clone2Dir, "push")

	// Create a divergent local commit and fetch.
	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "local divergent")
	gitRun(t, cloneDir, "fetch")

	// Test with a non-English error message — the english match should fail,
	// but the merge-base fallback should detect the divergence.
	nonEnglishErr := errors.New("erreur: impossible de fusionner")
	got := isNonFastForwardError(cloneDir, nonEnglishErr)
	if !got {
		t.Error("isNonFastForwardError should return true via merge-base fallback for diverged branches")
	}
}

func TestGitPush_NonOriginRemote(t *testing.T) {
	// Verify that GitPush uses the configured remote (not hardcoded "origin")
	// when a branch is configured with a different remote name.
	bareDir := t.TempDir()
	gitRun(t, bareDir, "init", "--bare")

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", bareDir, ".")
	gitRun(t, cloneDir, "config", "user.email", "test@test.com")
	gitRun(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "initial")
	gitRun(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Rename remote from "origin" to "upstream".
	gitRun(t, cloneDir, "remote", "rename", "origin", "upstream")

	// Create another commit to push.
	if err := os.WriteFile(filepath.Join(cloneDir, "file2.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cloneDir, "add", ".")
	gitRun(t, cloneDir, "commit", "-m", "second")

	svc := newTestService("test", cloneDir)
	result, pushErr := svc.GitPush("test")
	if pushErr != nil {
		t.Fatalf("GitPush failed: %v", pushErr)
	}
	if result.RemoteName != "upstream" {
		t.Fatalf("RemoteName = %q, want %q", result.RemoteName, "upstream")
	}
}

func TestGitPush_ConfigErrorAbortsPush(t *testing.T) {
	// Verify that GitPush aborts (instead of silently proceeding) when
	// git config returns an unexpected error (not exit code 1 "key not found").
	tmpDir := t.TempDir()
	_ = initGitRepo(t, tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmpDir, "add", ".")
	gitRun(t, tmpDir, "commit", "-m", "initial")

	// Read existing config, then append a corrupt section at the end.
	// git can still parse the [core] section (so rev-parse works), but
	// reading branch-specific keys hits the parse error.
	configPath := filepath.Join(tmpDir, ".git", "config")
	existingConfig, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	corruptConfig := string(existingConfig) + "\n[invalid-section\n"
	if err := os.WriteFile(configPath, []byte(corruptConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService("test", tmpDir)
	_, err := svc.GitPush("test")
	if err == nil {
		t.Fatal("expected error when git config is corrupt, but GitPush succeeded")
	}
}

// ---------------------------------------------------------------------------
// SearchFiles tests
// ---------------------------------------------------------------------------

func TestSearchFileResultFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4 // path, name, is_name_match, content_lines
	got := reflect.TypeFor[SearchFileResult]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("SearchFileResult field count = %d, want %d; update frontend fileTreeTypes.ts", got, expectedFieldCount)
	}
}

func TestSearchContentLineFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 2 // line, content
	got := reflect.TypeFor[SearchContentLine]().NumField()
	if got != expectedFieldCount {
		t.Fatalf("SearchContentLine field count = %d, want %d; update frontend fileTreeTypes.ts", got, expectedFieldCount)
	}
}

func TestSearchFiles(t *testing.T) {
	// Create a directory structure for testing.
	tmpDir := t.TempDir()
	for _, dir := range []string{"src", "src/sub", ".git", "node_modules"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"README.md":           "# Project Readme\nThis is a test project.",
		"main.go":             "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}",
		"src/app.go":          "package src\n\nfunc App() string {\n\treturn \"app\"\n}",
		"src/sub/util.go":     "package sub\n\nfunc Util() int {\n\treturn 42\n}",
		".git/config":         "[core]\nrepositoryformatversion = 0",
		"node_modules/pkg.js": "module.exports = {};",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := newTestService("test", tmpDir)

	tests := []struct {
		name    string
		session string
		query   string
		wantErr bool
		errMsg  string
		check   func(t *testing.T, results []SearchFileResult)
	}{
		{
			name:    "empty session name",
			session: "",
			query:   "main",
			wantErr: true,
			errMsg:  "session name is required",
		},
		{
			name:    "whitespace-only session name",
			session: "   ",
			query:   "main",
			wantErr: true,
			errMsg:  "session name is required",
		},
		{
			name:    "empty query returns empty slice",
			session: "test",
			query:   "",
			check: func(t *testing.T, results []SearchFileResult) {
				if len(results) != 0 {
					t.Fatalf("expected empty results, got %d", len(results))
				}
			},
		},
		{
			name:    "whitespace-only query returns empty slice",
			session: "test",
			query:   "   ",
			check: func(t *testing.T, results []SearchFileResult) {
				if len(results) != 0 {
					t.Fatalf("expected empty results, got %d", len(results))
				}
			},
		},
		{
			name:    "filename match case insensitive",
			session: "test",
			query:   "README",
			check: func(t *testing.T, results []SearchFileResult) {
				found := false
				for _, r := range results {
					if r.Path == "README.md" {
						found = true
						if !r.IsNameMatch {
							t.Error("expected IsNameMatch=true for README.md")
						}
					}
				}
				if !found {
					t.Error("expected README.md in results")
				}
			},
		},
		{
			name:    "filename match lowercase query",
			session: "test",
			query:   "readme",
			check: func(t *testing.T, results []SearchFileResult) {
				found := false
				for _, r := range results {
					if r.Path == "README.md" {
						found = true
					}
				}
				if !found {
					t.Error("expected README.md in results for lowercase query")
				}
			},
		},
		{
			name:    ".git directory excluded from results",
			session: "test",
			query:   "config",
			check: func(t *testing.T, results []SearchFileResult) {
				for _, r := range results {
					if strings.HasPrefix(r.Path, ".git/") {
						t.Errorf("expected .git/ to be excluded, got %s", r.Path)
					}
				}
			},
		},
		{
			name:    "node_modules excluded from results",
			session: "test",
			query:   "pkg",
			check: func(t *testing.T, results []SearchFileResult) {
				for _, r := range results {
					if strings.HasPrefix(r.Path, "node_modules/") {
						t.Errorf("expected node_modules/ to be excluded, got %s", r.Path)
					}
				}
			},
		},
		{
			name:    "results sorted by path",
			session: "test",
			query:   ".go",
			check: func(t *testing.T, results []SearchFileResult) {
				if len(results) < 2 {
					t.Fatalf("expected at least 2 results, got %d", len(results))
				}
				for i := 1; i < len(results); i++ {
					if results[i].Path < results[i-1].Path {
						t.Errorf("results not sorted: %s < %s", results[i].Path, results[i-1].Path)
					}
				}
			},
		},
		{
			name:    "invalid session returns error",
			session: "nonexistent",
			query:   "main",
			wantErr: true,
		},
		{
			name:    "content search finds matching lines",
			session: "test",
			query:   "hello",
			check: func(t *testing.T, results []SearchFileResult) {
				found := false
				for _, r := range results {
					if r.Path == "main.go" {
						for _, cl := range r.ContentLines {
							if strings.Contains(cl.Content, "hello") {
								found = true
								if cl.Line != 4 {
									t.Errorf("expected line 4, got %d", cl.Line)
								}
							}
						}
					}
				}
				if !found {
					t.Error("expected content match for 'hello' in main.go")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := svc.SearchFiles(tt.session, tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, results)
			}
		})
	}
}

func TestParseGitGrepOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		check   func(t *testing.T, results []SearchFileResult)
	}{
		{
			name:    "empty output",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "single match",
			input:   "src/main.go:10:func main() {\n",
			wantLen: 1,
			check: func(t *testing.T, results []SearchFileResult) {
				r := results[0]
				if r.Path != "src/main.go" {
					t.Errorf("path = %q, want %q", r.Path, "src/main.go")
				}
				if r.Name != "main.go" {
					t.Errorf("name = %q, want %q", r.Name, "main.go")
				}
				if len(r.ContentLines) != 1 {
					t.Fatalf("content lines = %d, want 1", len(r.ContentLines))
				}
				if r.ContentLines[0].Line != 10 {
					t.Errorf("line = %d, want 10", r.ContentLines[0].Line)
				}
				if r.ContentLines[0].Content != "func main() {" {
					t.Errorf("content = %q, want %q", r.ContentLines[0].Content, "func main() {")
				}
			},
		},
		{
			name:    "multiple matches same file grouped",
			input:   "app.go:1:line one\napp.go:5:line five\n",
			wantLen: 1,
			check: func(t *testing.T, results []SearchFileResult) {
				if len(results[0].ContentLines) != 2 {
					t.Errorf("content lines = %d, want 2", len(results[0].ContentLines))
				}
			},
		},
		{
			name:    "content with colons preserved",
			input:   "file.go:42:key: value: extra\n",
			wantLen: 1,
			check: func(t *testing.T, results []SearchFileResult) {
				if results[0].ContentLines[0].Content != "key: value: extra" {
					t.Errorf("content = %q, want %q", results[0].ContentLines[0].Content, "key: value: extra")
				}
			},
		},
		{
			name:    "max content lines per file enforced",
			input:   "a.go:1:l1\na.go:2:l2\na.go:3:l3\na.go:4:l4\n",
			wantLen: 1,
			check: func(t *testing.T, results []SearchFileResult) {
				if len(results[0].ContentLines) != maxContentLinesPerFile {
					t.Errorf("content lines = %d, want %d", len(results[0].ContentLines), maxContentLinesPerFile)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseGitGrepOutput([]byte(tt.input))
			if len(results) != tt.wantLen {
				t.Fatalf("result count = %d, want %d", len(results), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, results)
			}
		})
	}
}

func TestMergeSearchResults(t *testing.T) {
	nameMatches := []SearchFileResult{
		{Path: "a.go", Name: "a.go", IsNameMatch: true},
		{Path: "b.go", Name: "b.go", IsNameMatch: true},
	}
	contentMatches := []SearchFileResult{
		{Path: "b.go", Name: "b.go", ContentLines: []SearchContentLine{{Line: 5, Content: "match"}}},
		{Path: "c.go", Name: "c.go", ContentLines: []SearchContentLine{{Line: 1, Content: "other"}}},
	}

	merged := mergeSearchResults(nameMatches, contentMatches)

	if len(merged) != 3 {
		t.Fatalf("merged count = %d, want 3", len(merged))
	}

	// a.go: name match only
	if !merged[0].IsNameMatch || len(merged[0].ContentLines) != 0 {
		t.Errorf("a.go: expected name match only, got IsNameMatch=%v, lines=%d", merged[0].IsNameMatch, len(merged[0].ContentLines))
	}

	// b.go: merged (name + content)
	if !merged[1].IsNameMatch || len(merged[1].ContentLines) != 1 {
		t.Errorf("b.go: expected merged result, got IsNameMatch=%v, lines=%d", merged[1].IsNameMatch, len(merged[1].ContentLines))
	}

	// c.go: content match only
	if merged[2].IsNameMatch || len(merged[2].ContentLines) != 1 {
		t.Errorf("c.go: expected content match only, got IsNameMatch=%v, lines=%d", merged[2].IsNameMatch, len(merged[2].ContentLines))
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

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
