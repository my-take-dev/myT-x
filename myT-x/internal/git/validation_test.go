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
		{"contains double slash", "a//b", false},
		{"ends with .lock", "branch.lock", false},
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
		{"path traversal", "../hack", true},
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

func TestGenerateBranchName(t *testing.T) {
	result := GenerateBranchName("main", "feature")
	if !strings.HasPrefix(result, "main-feature-") {
		t.Errorf("GenerateBranchName() = %q, want prefix 'main-feature-'", result)
	}
	parts := strings.Split(result, "-")
	if len(parts) < 3 {
		t.Errorf("GenerateBranchName() = %q, expected at least 3 parts", result)
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
