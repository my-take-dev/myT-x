//go:build windows

package install

// NOTE: t.Setenv modifies process environment -- do not add t.Parallel() to these tests.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterPathEntries_RemovesMatchingEntry(t *testing.T) {
	tests := []struct {
		name      string
		pathValue string
		removeDir string
		want      string
	}{
		{
			name:      "removes single matching entry",
			pathValue: `C:\foo;C:\legacy\bin;C:\bar`,
			removeDir: `C:\legacy\bin`,
			want:      `C:\foo;C:\bar`,
		},
		{
			name:      "removes case-insensitively",
			pathValue: `C:\foo;C:\LEGACY\BIN;C:\bar`,
			removeDir: `C:\legacy\bin`,
			want:      `C:\foo;C:\bar`,
		},
		{
			name:      "removes duplicate entries",
			pathValue: `C:\foo;C:\legacy\bin;C:\bar;C:\legacy\bin`,
			removeDir: `C:\legacy\bin`,
			want:      `C:\foo;C:\bar`,
		},
		{
			name:      "preserves unrelated entries",
			pathValue: `C:\foo;C:\bar;C:\baz`,
			removeDir: `C:\legacy\bin`,
			want:      `C:\foo;C:\bar;C:\baz`,
		},
		{
			name:      "handles trailing semicolons",
			pathValue: `C:\foo;C:\legacy\bin;;C:\bar;`,
			removeDir: `C:\legacy\bin`,
			want:      `C:\foo;C:\bar`,
		},
		{
			name:      "empty path value",
			pathValue: "",
			removeDir: `C:\legacy\bin`,
			want:      "",
		},
		{
			name:      "only matching entry",
			pathValue: `C:\legacy\bin`,
			removeDir: `C:\legacy\bin`,
			want:      "",
		},
		{
			name:      "empty removeDir returns pathValue unchanged",
			pathValue: `C:\foo;.;C:\bar`,
			removeDir: "",
			want:      `C:\foo;.;C:\bar`,
		},
		{
			name:      "whitespace-only removeDir returns pathValue unchanged",
			pathValue: `C:\foo;C:\bar`,
			removeDir: "   ",
			want:      `C:\foo;C:\bar`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterPathEntries(tt.pathValue, tt.removeDir)
			if got != tt.want {
				t.Errorf("filterPathEntries(%q, %q) = %q, want %q",
					tt.pathValue, tt.removeDir, got, tt.want)
			}
		})
	}
}

func TestRemoveLegacyDirectory_RemovesKnownFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate legacy directory structure.
	legacyBase := filepath.Join(tmpDir, "github.com", "my-take-dev", "myT-x")
	binDir := filepath.Join(legacyBase, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// Create known files.
	knownFiles := []string{
		filepath.Join(binDir, "tmux.exe"),
		filepath.Join(binDir, "tmux.exe.sha256"),
		filepath.Join(legacyBase, "config.yaml"),
		filepath.Join(legacyBase, "shim-debug.log"),
		filepath.Join(legacyBase, "shim-debug-1234567890.log"),
	}
	for _, f := range knownFiles {
		if err := os.WriteFile(f, []byte("test"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", f, err)
		}
	}

	// Override LOCALAPPDATA for the test.
	t.Setenv("LOCALAPPDATA", tmpDir)

	removeLegacyDirectory(legacyBase, tmpDir)

	// Verify all known files were removed.
	for _, f := range knownFiles {
		if _, err := os.Stat(f); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("file %s still exists after cleanup", f)
		}
	}

	// Verify empty parent directories were cleaned up.
	if _, err := os.Stat(filepath.Join(tmpDir, "github.com")); !errors.Is(err, os.ErrNotExist) {
		t.Error("parent directory github.com still exists after cleanup")
	}
}

func TestRemoveLegacyDirectory_TrailingSeparatorNormalized(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a single-level directory under tmpDir to walk up.
	legacyBase := filepath.Join(tmpDir, "legacy-dir")
	if err := os.MkdirAll(legacyBase, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// Pass legacyBase with a trailing separator. filepath.Clean inside
	// removeLegacyDirectory normalizes this so os.Remove works correctly.
	legacyBaseWithTrailing := legacyBase + string(filepath.Separator)

	removeLegacyDirectory(legacyBaseWithTrailing, tmpDir)

	// Empty directory should have been removed despite the trailing separator.
	if _, err := os.Stat(legacyBase); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("directory %q still exists after cleanup with trailing separator", legacyBase)
	}
}

func TestRemoveLegacyDirectory_SkipsMissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("LOCALAPPDATA", tmpDir)

	nonExistent := filepath.Join(tmpDir, "nonexistent", "path")

	// Should not panic or error.
	removeLegacyDirectory(nonExistent, tmpDir)
}

func TestRemoveLegacyDirectory_PreservesUnknownFiles(t *testing.T) {
	tmpDir := t.TempDir()

	legacyBase := filepath.Join(tmpDir, "github.com", "my-take-dev", "myT-x")
	if err := os.MkdirAll(legacyBase, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	// Create an unknown file that should not be removed.
	unknownFile := filepath.Join(legacyBase, "user-data.txt")
	if err := os.WriteFile(unknownFile, []byte("important"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	t.Setenv("LOCALAPPDATA", tmpDir)

	removeLegacyDirectory(legacyBase, tmpDir)

	// Unknown file should still exist.
	if _, err := os.Stat(unknownFile); errors.Is(err, os.ErrNotExist) {
		t.Error("unknown file was unexpectedly removed")
	}

	// Directory should still exist because it's not empty.
	if _, err := os.Stat(legacyBase); errors.Is(err, os.ErrNotExist) {
		t.Error("legacy directory was removed even though it contains unknown files")
	}
}

func TestRemoveProcessPathEntry_RemovesEntry(t *testing.T) {
	testDir := `C:\test-legacy-shim-dir-12345`
	t.Setenv("PATH", os.Getenv("PATH")+";"+testDir)

	removed := removeProcessPathEntry(testDir)
	if !removed {
		t.Fatal("removeProcessPathEntry returned false, expected true")
	}

	newPath := os.Getenv("PATH")
	if strings.Contains(strings.ToLower(newPath), strings.ToLower(testDir)) {
		t.Errorf("PATH still contains %q after removal", testDir)
	}
}

func TestRemoveProcessPathEntry_NoOpWhenAbsent(t *testing.T) {
	t.Setenv("PATH", os.Getenv("PATH"))

	removed := removeProcessPathEntry(`C:\nonexistent-dir-12345`)
	if removed {
		t.Fatal("removeProcessPathEntry returned true for absent entry")
	}
}

func TestRemoveProcessPathEntry_EmptyDir(t *testing.T) {
	removed := removeProcessPathEntry("")
	if removed {
		t.Fatal("removeProcessPathEntry returned true for empty dir")
	}
}

func TestCleanupLegacyShimInstalls_IdempotentWhenNoLegacy(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("LOCALAPPDATA", tmpDir)
	t.Setenv("PATH", os.Getenv("PATH"))

	// CleanupLegacyShimInstalls should not error even when
	// legacy directories don't exist.
	if err := CleanupLegacyShimInstalls(); err != nil {
		t.Fatalf("CleanupLegacyShimInstalls() error = %v", err)
	}
}

func TestCleanupLegacyShimInstalls_GracefulWhenLOCALAPPDATAEmpty(t *testing.T) {
	// When LOCALAPPDATA is empty, CleanupLegacyShimInstalls must return nil
	// without panicking or modifying anything. This tests graceful degradation
	// when the environment is incomplete.
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("PATH", os.Getenv("PATH"))

	if err := CleanupLegacyShimInstalls(); err != nil {
		t.Fatalf("CleanupLegacyShimInstalls() error = %v, want nil", err)
	}
}

func TestFilterStalePathEntries(t *testing.T) {
	tests := []struct {
		name             string
		pathValue        string
		substring        string
		wantResult       string
		wantRemovedCount int
	}{
		{
			name:             "removes single stale entry",
			pathValue:        `C:\Windows\System32;C:\Users\test\AppData\Local\Temp\TestEnsureShim12345\bin;C:\tools`,
			substring:        stalePathSubstring,
			wantResult:       `C:\Windows\System32;C:\tools`,
			wantRemovedCount: 1,
		},
		{
			name:             "preserves non-stale entries",
			pathValue:        `C:\Windows\System32;C:\Program Files\Go\bin;C:\tools`,
			substring:        stalePathSubstring,
			wantResult:       `C:\Windows\System32;C:\Program Files\Go\bin;C:\tools`,
			wantRemovedCount: 0,
		},
		{
			name:             "empty pathValue returns empty unchanged",
			pathValue:        "",
			substring:        stalePathSubstring,
			wantResult:       "",
			wantRemovedCount: 0,
		},
		{
			name:             "empty substring returns pathValue unchanged",
			pathValue:        `C:\Windows\System32;C:\tools`,
			substring:        "",
			wantResult:       `C:\Windows\System32;C:\tools`,
			wantRemovedCount: 0,
		},
		{
			name:             "empty substring with empty pathValue",
			pathValue:        "",
			substring:        "",
			wantResult:       "",
			wantRemovedCount: 0,
		},
		{
			name:             "multiple stale entries all removed",
			pathValue:        `C:\keep;C:\Temp\TestEnsureShim111\bin;C:\also-keep;C:\Temp\TestEnsureShim222\bin;C:\final`,
			substring:        stalePathSubstring,
			wantResult:       `C:\keep;C:\also-keep;C:\final`,
			wantRemovedCount: 2,
		},
		{
			name:             "case-insensitive match uppercase",
			pathValue:        `C:\keep;C:\TEMP\TESTENSSURESHIM999\BIN;C:\other`,
			substring:        `\temp\testenssureshim`,
			wantResult:       `C:\keep;C:\other`,
			wantRemovedCount: 1,
		},
		{
			name:             "case-insensitive match mixed case substring",
			pathValue:        `C:\foo;C:\Users\Temp\TestEnsureShim456\bin;C:\bar`,
			substring:        `\temp\testenssureshim`,
			wantResult:       `C:\foo;C:\Users\Temp\TestEnsureShim456\bin;C:\bar`,
			wantRemovedCount: 0,
		},
		{
			name:             "semicolon-only pathValue",
			pathValue:        ";",
			substring:        stalePathSubstring,
			wantResult:       "",
			wantRemovedCount: 0,
		},
		{
			name:             "double semicolons pathValue",
			pathValue:        ";;",
			substring:        stalePathSubstring,
			wantResult:       "",
			wantRemovedCount: 0,
		},
		{
			name:             "trailing semicolons with stale entry",
			pathValue:        `C:\keep;C:\Temp\TestEnsureShim789\bin;;`,
			substring:        stalePathSubstring,
			wantResult:       `C:\keep`,
			wantRemovedCount: 1,
		},
		{
			name:             "all entries are stale",
			pathValue:        `C:\Temp\TestEnsureShim001\bin;C:\Temp\TestEnsureShim002\bin`,
			substring:        stalePathSubstring,
			wantResult:       "",
			wantRemovedCount: 2,
		},
		{
			name:             "whitespace-padded entries trimmed",
			pathValue:        `  C:\keep  ; C:\Temp\TestEnsureShim555\bin ; C:\other  `,
			substring:        stalePathSubstring,
			wantResult:       `C:\keep;C:\other`,
			wantRemovedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotCount := filterStalePathEntries(tt.pathValue, tt.substring)
			if gotResult != tt.wantResult {
				t.Errorf("filterStalePathEntries(%q, %q) result = %q, want %q",
					tt.pathValue, tt.substring, gotResult, tt.wantResult)
			}
			if gotCount != tt.wantRemovedCount {
				t.Errorf("filterStalePathEntries(%q, %q) removedCount = %d, want %d",
					tt.pathValue, tt.substring, gotCount, tt.wantRemovedCount)
			}
		})
	}
}

func TestCleanupLegacyShimInstalls_RemovesLegacyFromProcessPath(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("LOCALAPPDATA", tmpDir)

	legacyBinDir := filepath.Join(tmpDir, "github.com", "my-take-dev", "myT-x", "bin")

	t.Setenv("PATH", os.Getenv("PATH")+";"+legacyBinDir)

	if err := CleanupLegacyShimInstalls(); err != nil {
		t.Fatalf("CleanupLegacyShimInstalls() error = %v", err)
	}

	newPath := os.Getenv("PATH")
	if containsPathEntry(newPath, legacyBinDir) {
		t.Errorf("process PATH still contains legacy entry %q", legacyBinDir)
	}
}
