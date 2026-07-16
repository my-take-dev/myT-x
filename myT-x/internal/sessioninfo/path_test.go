package sessioninfo

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFolderKey(t *testing.T) {
	base := filepath.Join(t.TempDir(), "Project")
	cleanedVariant := filepath.Join(base, ".", "sub", "..")
	baseKey, err := FolderKey(base)
	if err != nil {
		t.Fatalf("FolderKey(base): %v", err)
	}
	cleanedKey, err := FolderKey(cleanedVariant)
	if err != nil {
		t.Fatalf("FolderKey(cleanedVariant): %v", err)
	}
	if cleanedKey != baseKey {
		t.Fatalf("cleaned equivalent path key = %q, want %q", cleanedKey, baseKey)
	}

	caseVariant := strings.ToUpper(base)
	caseKey, err := FolderKey(caseVariant)
	if err != nil {
		t.Fatalf("FolderKey(caseVariant): %v", err)
	}
	if runtime.GOOS == "windows" && caseKey != baseKey {
		t.Fatalf("Windows case-varied path key = %q, want %q", caseKey, baseKey)
	}
	if runtime.GOOS == "darwin" && caseKey != baseKey {
		t.Fatalf("Darwin case-varied path key = %q, want %q", caseKey, baseKey)
	}

	otherKey, err := FolderKey(base + "-other")
	if err != nil {
		t.Fatalf("FolderKey(other): %v", err)
	}
	if otherKey == baseKey {
		t.Fatal("distinct folders mapped to the same key")
	}
}

func TestFolderKeyRejectsEmptyWorkDir(t *testing.T) {
	if _, err := FolderKey("   "); err == nil {
		t.Fatal("FolderKey() expected empty workDir error")
	}
}

func TestFilePath(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	workDir := filepath.Join(t.TempDir(), "workspace")
	key, err := FolderKey(workDir)
	if err != nil {
		t.Fatalf("FolderKey(): %v", err)
	}

	got, err := FilePath(configDir, workDir, "session-memo.md")
	if err != nil {
		t.Fatalf("FilePath(): %v", err)
	}
	want := filepath.Join(configDir, DirName, key, "session-memo.md")
	if got != want {
		t.Fatalf("FilePath() = %q, want %q", got, want)
	}
}

func TestDirectoryPath(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	workDir := filepath.Join(t.TempDir(), "workspace")
	key, err := FolderKey(workDir)
	if err != nil {
		t.Fatalf("FolderKey(): %v", err)
	}

	got, err := DirectoryPath(configDir, workDir)
	if err != nil {
		t.Fatalf("DirectoryPath(): %v", err)
	}
	want := filepath.Join(configDir, DirName, key)
	if got != want {
		t.Fatalf("DirectoryPath() = %q, want %q", got, want)
	}
}

func TestFilePathRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		configDir string
		workDir   string
		fileName  string
	}{
		{
			name:      "empty config dir",
			configDir: " ",
			workDir:   t.TempDir(),
			fileName:  "session-memo.md",
		},
		{
			name:      "empty work dir",
			configDir: t.TempDir(),
			workDir:   " ",
			fileName:  "session-memo.md",
		},
		{
			name:      "empty file name",
			configDir: t.TempDir(),
			workDir:   t.TempDir(),
			fileName:  " ",
		},
		{
			name:      "parent traversal file name",
			configDir: t.TempDir(),
			workDir:   t.TempDir(),
			fileName:  "..",
		},
		{
			name:      "nested slash file name",
			configDir: t.TempDir(),
			workDir:   t.TempDir(),
			fileName:  "nested/session-memo.md",
		},
		{
			name:      "nested backslash file name",
			configDir: t.TempDir(),
			workDir:   t.TempDir(),
			fileName:  `nested\session-memo.md`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FilePath(tt.configDir, tt.workDir, tt.fileName); err == nil {
				t.Fatal("FilePath() expected error")
			}
		})
	}
}

func TestLegacyProjectFilePath(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "workspace")

	got, err := LegacyProjectFilePath(workDir, "session-memo.md")
	if err != nil {
		t.Fatalf("LegacyProjectFilePath(): %v", err)
	}
	normalizedWorkDir := filepath.Clean(workDir)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		normalizedWorkDir = strings.ToLower(normalizedWorkDir)
	}
	want := filepath.Join(normalizedWorkDir, ".myT-x", "session-memo.md")
	if got != want {
		t.Fatalf("LegacyProjectFilePath() = %q, want %q", got, want)
	}
}
