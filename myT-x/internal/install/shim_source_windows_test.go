//go:build windows

package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindShimSourceAdjacentToExecutable(t *testing.T) {
	// When tmux-shim.exe exists next to os.Executable(), findShimSource returns it.
	exePath, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable() failed, skipping test")
	}
	exeDir := filepath.Dir(exePath)
	candidate := filepath.Join(exeDir, "tmux-shim.exe")

	// Create a dummy shim adjacent to the test binary.
	if err := os.WriteFile(candidate, []byte("dummy-shim"), 0o755); err != nil {
		t.Fatalf("failed to create dummy shim: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(candidate)
	})

	got, err := findShimSource("")
	if err != nil {
		t.Fatalf("findShimSource() error = %v", err)
	}
	if got != candidate {
		t.Fatalf("findShimSource() = %q, want %q", got, candidate)
	}
}

func TestFindShimSourceWorkspaceRootEmpty(t *testing.T) {
	// When os.Executable() dir has no tmux-shim.exe and workspaceRoot is empty,
	// findShimSource returns "source not found" error.
	exePath, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable() failed, skipping test")
	}
	// Ensure no adjacent shim exists.
	candidate := filepath.Join(filepath.Dir(exePath), "tmux-shim.exe")
	if fileExists(candidate) {
		t.Skip("tmux-shim.exe already exists next to test binary, skipping")
	}

	_, err = findShimSource("")
	if err == nil {
		t.Fatal("findShimSource(\"\") expected error when no source found")
	}
}

func TestFindShimSourceWorkspaceRootNoGoMod(t *testing.T) {
	// When workspaceRoot has no go.mod, it should not attempt go build.
	exePath, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable() failed, skipping test")
	}
	candidate := filepath.Join(filepath.Dir(exePath), "tmux-shim.exe")
	if fileExists(candidate) {
		t.Skip("tmux-shim.exe already exists next to test binary, skipping")
	}

	tmpDir := t.TempDir()
	_, err = findShimSource(tmpDir)
	if err == nil {
		t.Fatal("findShimSource(noGoMod) expected error")
	}
	// Should get "not found" error, not "build failed" error.
	if got := err.Error(); got != "tmux-shim.exe source not found" {
		t.Fatalf("findShimSource() error = %q, want 'tmux-shim.exe source not found'", got)
	}
}

func TestCopyFile(t *testing.T) {
	t.Run("copies file contents correctly", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		src := filepath.Join(srcDir, "source.exe")
		dst := filepath.Join(dstDir, "dest.exe")

		content := []byte("binary-content-here")
		if err := os.WriteFile(src, content, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile() error = %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read destination: %v", err)
		}
		if string(got) != string(content) {
			t.Fatalf("destination content = %q, want %q", string(got), string(content))
		}
	})

	t.Run("returns error for nonexistent source", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "dest.exe")
		if err := copyFile(filepath.Join(t.TempDir(), "missing.exe"), dst); err == nil {
			t.Fatal("copyFile() expected error for missing source")
		}
		// Destination should not exist.
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			t.Fatalf("destination should not exist after failed copy, stat err = %v", err)
		}
	})

	t.Run("overwrites existing destination atomically", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		src := filepath.Join(srcDir, "new.exe")
		dst := filepath.Join(dstDir, "existing.exe")

		// Create existing destination.
		if err := os.WriteFile(dst, []byte("old-content"), 0o755); err != nil {
			t.Fatal(err)
		}

		// Create new source.
		if err := os.WriteFile(src, []byte("new-content"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile() error = %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read destination: %v", err)
		}
		if string(got) != "new-content" {
			t.Fatalf("destination content = %q, want %q", string(got), "new-content")
		}
	})
}

func TestFileExists(t *testing.T) {
	tests := []struct {
		name string
		want bool
		setup func(t *testing.T) string
	}{
		{
			name: "existing file returns true",
			want: true,
			setup: func(t *testing.T) string {
				t.Helper()
				f := filepath.Join(t.TempDir(), "exists.txt")
				if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
					t.Fatal(err)
				}
				return f
			},
		},
		{
			name: "nonexistent file returns false",
			want: false,
			setup: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "missing.txt")
			},
		},
		{
			name: "directory returns false",
			want: false,
			setup: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			if got := fileExists(path); got != tt.want {
				t.Fatalf("fileExists(%q) = %v, want %v", path, got, tt.want)
			}
		})
	}
}
