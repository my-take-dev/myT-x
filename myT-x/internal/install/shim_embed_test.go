//go:build windows

package install

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestHasEmbeddedShim_Stub(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	embeddedShimBinary = nil
	if HasEmbeddedShim() {
		t.Fatal("HasEmbeddedShim() = true, want false when binary is nil")
	}
}

func TestHasEmbeddedShim_WithData(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	embeddedShimBinary = []byte("fake-shim-binary")
	if !HasEmbeddedShim() {
		t.Fatal("HasEmbeddedShim() = false, want true when binary is set")
	}
}

func TestGetEmbeddedShim_Stub(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	embeddedShimBinary = nil
	if got := GetEmbeddedShim(); got != nil {
		t.Fatalf("GetEmbeddedShim() = %v, want nil", got)
	}
}

func TestGetEmbeddedShim_WithData(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	want := []byte("fake-shim-binary-data")
	embeddedShimBinary = want
	got := GetEmbeddedShim()
	if string(got) != string(want) {
		t.Fatalf("GetEmbeddedShim() = %q, want %q", got, want)
	}
}

// setupEmbedTestEnv sets up LOCALAPPDATA and PATH for shim install tests and
// redirects PATH mutation to process-local updates (no registry writes).
// Returns the installDir path.
// NOTE: This helper mutates package-level test seams; callers must not use
// t.Parallel() in tests that invoke this helper.
func setupEmbedTestEnv(t *testing.T) string {
	t.Helper()

	origEnsurePathContains := ensurePathContainsFn
	t.Cleanup(func() { ensurePathContainsFn = origEnsurePathContains })
	ensurePathContainsFn = func(dir string) (bool, error) {
		return EnsureProcessPathContains(dir), nil
	}

	origLocal := os.Getenv("LOCALAPPDATA")
	t.Cleanup(func() { _ = os.Setenv("LOCALAPPDATA", origLocal) })
	tempDir := t.TempDir()
	if err := os.Setenv("LOCALAPPDATA", tempDir); err != nil {
		t.Fatalf("Setenv(LOCALAPPDATA) error = %v", err)
	}

	installDir := filepath.Join(tempDir, "myT-x", "bin")
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", origPath+";"+installDir); err != nil {
		t.Fatalf("Setenv(PATH) error = %v", err)
	}

	return installDir
}

func TestEnsureShimInstalled_EmbeddedPath(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	fakeShimContent := []byte("MZ-fake-embedded-shim-binary")
	embeddedShimBinary = fakeShimContent

	installDir := setupEmbedTestEnv(t)

	result, err := EnsureShimInstalled("")
	if err != nil {
		t.Fatalf("EnsureShimInstalled() error = %v", err)
	}

	target := filepath.Join(installDir, "tmux.exe")
	if result.InstalledPath != target {
		t.Fatalf("InstalledPath = %q, want %q", result.InstalledPath, target)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", target, err)
	}
	if string(got) != string(fakeShimContent) {
		t.Fatalf("installed file content = %q, want %q", got, fakeShimContent)
	}

	// Verify hash file was written alongside tmux.exe.
	hashFile := target + ".sha256"
	hashBytes, err := os.ReadFile(hashFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", hashFile, err)
	}
	wantHash := sha256Hex(fakeShimContent)
	if string(hashBytes) != wantHash {
		t.Fatalf("hash file content = %q, want %q", hashBytes, wantHash)
	}
}

func TestEnsureShimInstalled_FallbackWhenNoEmbed(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	embeddedShimBinary = nil

	installDir := setupEmbedTestEnv(t)

	// Create a fake shim source next to the test executable's directory.
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	shimSource := filepath.Join(filepath.Dir(exePath), "tmux-shim.exe")
	fakeContent := []byte("MZ-fake-file-based-shim")
	if err := os.WriteFile(shimSource, fakeContent, 0o755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", shimSource, err)
	}
	t.Cleanup(func() { _ = os.Remove(shimSource) })

	result, err := EnsureShimInstalled("")
	if err != nil {
		t.Fatalf("EnsureShimInstalled() error = %v", err)
	}

	target := filepath.Join(installDir, "tmux.exe")
	if result.InstalledPath != target {
		t.Fatalf("InstalledPath = %q, want %q", result.InstalledPath, target)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", target, err)
	}
	if string(got) != string(fakeContent) {
		t.Fatalf("installed file content = %q, want %q", got, fakeContent)
	}

	// Verify hash file was written.
	hashFile := target + ".sha256"
	hashBytes, err := os.ReadFile(hashFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", hashFile, err)
	}
	wantHash := sha256Hex(fakeContent)
	if string(hashBytes) != wantHash {
		t.Fatalf("hash file content = %q, want %q", hashBytes, wantHash)
	}
}

func TestEnsureShimInstalled_SkipsWhenHashMatches(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	fakeShimContent := []byte("MZ-embedded-shim-v1")
	embeddedShimBinary = fakeShimContent

	installDir := setupEmbedTestEnv(t)

	// First install: writes tmux.exe + hash file.
	if _, err := EnsureShimInstalled(""); err != nil {
		t.Fatalf("first EnsureShimInstalled() error = %v", err)
	}

	target := filepath.Join(installDir, "tmux.exe")
	info1, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", target, err)
	}
	modTime1 := info1.ModTime()

	// Second install with same content: should skip write.
	if _, err := EnsureShimInstalled(""); err != nil {
		t.Fatalf("second EnsureShimInstalled() error = %v", err)
	}

	info2, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(%s) after skip error = %v", target, err)
	}
	if !info2.ModTime().Equal(modTime1) {
		t.Fatalf("tmux.exe was rewritten when hash matched (modtime changed: %v -> %v)", modTime1, info2.ModTime())
	}
}

func TestEnsureShimInstalled_OverwritesWhenHashDiffers(t *testing.T) {
	saved := embeddedShimBinary
	t.Cleanup(func() { embeddedShimBinary = saved })

	v1Content := []byte("MZ-embedded-shim-v1")
	embeddedShimBinary = v1Content

	installDir := setupEmbedTestEnv(t)

	// First install.
	if _, err := EnsureShimInstalled(""); err != nil {
		t.Fatalf("first EnsureShimInstalled() error = %v", err)
	}

	target := filepath.Join(installDir, "tmux.exe")

	// Change embedded content to v2.
	v2Content := []byte("MZ-embedded-shim-v2-updated")
	embeddedShimBinary = v2Content

	// Second install: should overwrite.
	if _, err := EnsureShimInstalled(""); err != nil {
		t.Fatalf("second EnsureShimInstalled() error = %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", target, err)
	}
	if string(got) != string(v2Content) {
		t.Fatalf("after overwrite, content = %q, want %q", got, v2Content)
	}

	// Verify hash file was also updated.
	hashFile := target + ".sha256"
	hashBytes, err := os.ReadFile(hashFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", hashFile, err)
	}
	wantHash := sha256Hex(v2Content)
	if string(hashBytes) != wantHash {
		t.Fatalf("hash file content = %q, want %q", hashBytes, wantHash)
	}
}

func TestSha256Hex(t *testing.T) {
	input := []byte("hello world")
	got := sha256Hex(input)
	// Known SHA256 of "hello world"
	h := sha256.Sum256(input)
	want := hex.EncodeToString(h[:])
	if got != want {
		t.Fatalf("sha256Hex(%q) = %q, want %q", input, got, want)
	}
	if len(got) != 64 {
		t.Fatalf("sha256Hex result length = %d, want 64", len(got))
	}
}

func TestSha256HexFile(t *testing.T) {
	content := []byte("test file content for sha256")
	tmpFile := filepath.Join(t.TempDir(), "test.bin")
	if err := os.WriteFile(tmpFile, content, 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	got, err := sha256HexFile(tmpFile)
	if err != nil {
		t.Fatalf("sha256HexFile() error = %v", err)
	}

	want := sha256Hex(content)
	if got != want {
		t.Fatalf("sha256HexFile() = %q, want %q", got, want)
	}
}

func TestSha256HexFile_NotFound(t *testing.T) {
	_, err := sha256HexFile(filepath.Join(t.TempDir(), "nonexistent.bin"))
	if err == nil {
		t.Fatal("sha256HexFile() expected error for nonexistent file")
	}
}

func TestMatchesHashFile(t *testing.T) {
	tests := []struct {
		name     string
		stored   string
		expected string
		want     bool
	}{
		{"exact match", "abc123", "abc123", true},
		{"with trailing newline", "abc123\n", "abc123", true},
		{"with trailing spaces", "abc123  ", "abc123", true},
		{"mismatch", "abc123", "def456", false},
		{"empty stored", "", "abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashFile := filepath.Join(t.TempDir(), "test.sha256")
			if err := os.WriteFile(hashFile, []byte(tt.stored), 0o644); err != nil {
				t.Fatalf("WriteFile error = %v", err)
			}
			if got := matchesHashFile(hashFile, tt.expected); got != tt.want {
				t.Fatalf("matchesHashFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesHashFile_NotFound(t *testing.T) {
	if matchesHashFile(filepath.Join(t.TempDir(), "nonexistent.sha256"), "abc123") {
		t.Fatal("matchesHashFile() should return false for nonexistent file")
	}
}
