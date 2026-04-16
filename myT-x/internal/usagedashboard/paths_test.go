package usagedashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbsPathToClaudeSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"windows drive hyphens", `D:\myT-x\dev-myT-x`, "D--myT-x-dev-myT-x"},
		{"underscores replaced", `D:\test_repository\test_repo`, "D--test-repository-test-repo"},
		{"mixed hyphen and underscore", `D:\test_repository\test_repo_dbw`, "D--test-repository-test-repo-dbw"},
		{"dot segment becomes hyphen", `C:\Users\mytakedev\.claude-mem`, "C--Users-mytakedev--claude-mem"},
		{"space becomes hyphen", `C:\Users\foo\bar baz`, "C--Users-foo-bar-baz"},
		{"posix path", "/home/user/proj", "-home-user-proj"},
		{"trailing sep", `D:\foo\bar\`, "D--foo-bar"},
		{"drive only", `D:\`, "D--"},
		{"empty", "", ""},
		{"dot only", ".", ""},
		{"mixed separators", `D:/foo\bar`, "D--foo-bar"},
		{"unicode passthrough filtered", `D:\プロジェクト`, "D--------"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AbsPathToClaudeSlug(tc.in)
			if got != tc.want {
				t.Errorf("AbsPathToClaudeSlug(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPathsEqualFold(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"same case", `D:\foo\bar`, `D:\foo\bar`, true},
		{"different drive case", `d:\foo\bar`, `D:\foo\bar`, true},
		{"trailing slash", `D:\foo\bar\`, `D:\foo\bar`, true},
		{"different path", `D:\foo\bar`, `D:\foo\baz`, false},
		{"both empty", "", "", false},
		{"one empty", "", `D:\foo`, false},
		{"whitespace", `  D:\foo\bar  `, `D:\foo\bar`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PathsEqualFold(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("PathsEqualFold(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestFindClaudeProjectDir(t *testing.T) {
	claudeHome := t.TempDir()
	projectsDir := filepath.Join(claudeHome, "projects")
	slug := "D--myT-x-dev-myT-x"
	if err := os.MkdirAll(filepath.Join(projectsDir, slug), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("exact match", func(t *testing.T) {
		got, err := FindClaudeProjectDir(claudeHome, `D:\myT-x\dev-myT-x`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(projectsDir, slug)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := FindClaudeProjectDir(claudeHome, `D:\does\not\exist`); err == nil {
			t.Error("expected error for missing slug, got nil")
		}
	})

	t.Run("case-insensitive fallback", func(t *testing.T) {
		fallbackHome := t.TempDir()
		fallbackProjectsDir := filepath.Join(fallbackHome, "projects")
		fallbackSlug := strings.ToLower(slug)
		if err := os.MkdirAll(filepath.Join(fallbackProjectsDir, fallbackSlug), 0o755); err != nil {
			t.Fatalf("setup fallback slug: %v", err)
		}

		got, err := FindClaudeProjectDir(fallbackHome, `D:\myT-x\dev-myT-x`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(fallbackProjectsDir, fallbackSlug)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		if _, err := FindClaudeProjectDir(claudeHome, ""); err == nil {
			t.Error("expected error for empty path, got nil")
		}
	})

	t.Run("missing projects dir", func(t *testing.T) {
		missingHome := t.TempDir()
		if _, err := FindClaudeProjectDir(missingHome, `D:\foo`); err == nil {
			t.Error("expected error for missing projects dir, got nil")
		}
	})
}

func TestResolveClaudeHome(t *testing.T) {
	home := t.TempDir()
	got, err := ResolveClaudeHome(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".claude")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveCodexHome(t *testing.T) {
	home := t.TempDir()
	got, err := ResolveCodexHome(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".codex")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
