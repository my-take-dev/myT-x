package git

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsValidBranchName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "main", true},
		{"valid with slash", "feature/auth", true},
		{"valid with hyphen", "fix-bug", true},
		{"valid with underscore", "my_branch", true},
		{"valid with dot", "v1.0", true},
		{"valid nested slash", "feature/auth/login", true},
		{"empty", "", false},
		{"starts with dot", ".hidden", false},
		{"starts with hyphen", "-bad", false},
		{"starts with slash", "/bad", false},
		{"ends with slash", "bad/", false},
		{"ends with dot", "bad.", false},
		{"contains double dot", "a..b", false},
		{"path traversal via slash", "a/../b", false},
		{"contains double slash", "a//b", false},
		{"ends with .lock", "branch.lock", false},
		{"contains null", "a\x00b", false},
		{"special chars", "a@b", false},
		{"space", "a b", false},
		{"backslash", `a\b`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidBranchName(tt.input)
			if got != tt.want {
				t.Errorf("IsValidBranchName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "main", false},
		{"empty", "", true},
		{"invalid chars", "a@b", true},
		{"contains null", "a\x00b", true},
		{"path traversal", "../hack", true},
		{"path traversal via slash", "a/../b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBranchName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCommitish(t *testing.T) {
	tests := []struct {
		name      string
		commitish string
		wantErr   bool
	}{
		{name: "HEAD", commitish: "HEAD", wantErr: false},
		{name: "commit hash", commitish: "a1b2c3d4", wantErr: false},
		{name: "ref with slash", commitish: "origin/main", wantErr: false},
		{name: "HEAD with tilde", commitish: "HEAD~1", wantErr: false},
		{name: "HEAD with caret", commitish: "HEAD^", wantErr: false},
		{name: "empty", commitish: "", wantErr: true},
		{name: "contains space", commitish: "main branch", wantErr: true},
		{name: "contains null", commitish: "a\x00b", wantErr: true},
		{name: "semicolon injection", commitish: "HEAD;echo", wantErr: true},
		{name: "pipe injection", commitish: "HEAD|cat", wantErr: true},
		{name: "command substitution", commitish: "$(cmd)", wantErr: true},
		{name: "backtick substitution", commitish: "`whoami`", wantErr: true},
		{name: "newline injection", commitish: "HEAD\nmalicious", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommitish(tt.commitish)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCommitish(%q) error = %v, wantErr %v", tt.commitish, err, tt.wantErr)
			}
		})
	}

	t.Run("reports escaped invalid value and allowed pattern", func(t *testing.T) {
		err := ValidateCommitish("bad value")
		if err == nil {
			t.Fatal("ValidateCommitish() expected error for invalid commit-ish")
		}
		if !strings.Contains(err.Error(), `"bad value"`) {
			t.Fatalf("error = %v, want quoted invalid value", err)
		}
		if !strings.Contains(err.Error(), "allowed pattern:") {
			t.Fatalf("error = %v, want allowed pattern hint", err)
		}
	})
}

func TestSanitizeCustomName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "feature", "feature"},
		{"with spaces", "my feature", "myfeature"},
		{"with special", "my@feature!", "myfeature"},
		{"uppercase", "MyBranch", "mybranch"},
		{"empty", "", "work"},
		{"only special", "@#$", "work"},
		{"hyphens ok", "my-branch", "my-branch"},
		{"underscores ok", "my_branch", "my_branch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeCustomName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeCustomName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateWorktreeDirPath(t *testing.T) {
	var tests []struct {
		name     string
		repoPath string
		want     string
	}

	if runtime.GOOS == "windows" {
		tests = []struct {
			name     string
			repoPath string
			want     string
		}{
			{"simple path", `C:\Projects\myapp`, `C:\Projects\myapp.wt`},
			{"nested path", `C:\Users\dev\repos\backend`, `C:\Users\dev\repos\backend.wt`},
		}
	} else {
		tests = []struct {
			name     string
			repoPath string
			want     string
		}{
			{"simple path", "/home/user/projects/myapp", "/home/user/projects/myapp.wt"},
			{"nested path", "/home/user/dev/repos/backend", "/home/user/dev/repos/backend.wt"},
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateWorktreeDirPath(tt.repoPath)
			if got != tt.want {
				t.Errorf("GenerateWorktreeDirPath(%q) = %q, want %q", tt.repoPath, got, tt.want)
			}
		})
	}
}

func TestGenerateWorktreePath(t *testing.T) {
	var repoPath, want string
	if runtime.GOOS == "windows" {
		repoPath = `C:\Projects\myapp`
		want = `C:\Projects\myapp.wt\feature-auth`
	} else {
		repoPath = "/home/user/projects/myapp"
		want = "/home/user/projects/myapp.wt/feature-auth"
	}
	got := GenerateWorktreePath(repoPath, "feature-auth")
	if got != want {
		t.Errorf("GenerateWorktreePath() = %q, want %q", got, want)
	}
}

func TestValidateWorktreePath(t *testing.T) {
	var absPath string
	var gitDirPath string
	if runtime.GOOS == "windows" {
		absPath = `C:\Projects\myapp.wt\feature`
		gitDirPath = `C:\Projects\.git`
	} else {
		absPath = "/home/user/projects/myapp.wt/feature"
		gitDirPath = "/home/user/projects/.git"
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute", absPath, false},
		{"valid absolute with double-dot segment text", filepath.Join(filepath.Dir(absPath), "my..project"), false},
		{"empty", "", true},
		{"relative", "relative/path", true},
		{"path traversal", absPath + string(filepath.Separator) + ".." + string(filepath.Separator) + "hack", true},
		{"git dir", gitDirPath, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorktreePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorktreePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestFindAvailableWorktreePath(t *testing.T) {
	t.Run("non-existing path returns as-is", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "nonexistent")
		got := FindAvailableWorktreePath(base)
		if got != base {
			t.Errorf("FindAvailableWorktreePath(%q) = %q, want %q", base, got, base)
		}
	})

	t.Run("existing path returns suffixed", func(t *testing.T) {
		dir := t.TempDir()
		base := filepath.Join(dir, "myworktree")
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}

		got := FindAvailableWorktreePath(base)
		want := base + "-2"
		if got != want {
			t.Errorf("FindAvailableWorktreePath(%q) = %q, want %q", base, got, want)
		}
	})

	t.Run("multiple existing paths skip to next free", func(t *testing.T) {
		dir := t.TempDir()
		base := filepath.Join(dir, "myworktree")
		for _, suffix := range []string{"", "-2", "-3"} {
			if err := os.MkdirAll(base+suffix, 0o755); err != nil {
				t.Fatal(err)
			}
		}

		got := FindAvailableWorktreePath(base)
		want := base + "-4"
		if got != want {
			t.Errorf("FindAvailableWorktreePath(%q) = %q, want %q", base, got, want)
		}
	})
}
