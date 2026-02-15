package testutil

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SkipIfNoGit skips the test if git is not available.
func SkipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}
}

// ResolvePath resolves Windows 8.3 short paths (e.g., MYTAKE~1 -> mytakedev)
// so that paths match git's output which always uses long path names.
// Note: filepath.EvalSymlinks on Windows also resolves 8.3 short names as a side effect,
// which is the primary purpose here (not symlink resolution).
// Returns the original path if resolution fails.
func ResolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		slog.Debug("[DEBUG-TEST] EvalSymlinks failed, using original path",
			"path", path, "error", err)
		return path
	}
	return resolved
}

// CreateTempGitRepo creates a temporary git repository for testing.
// The returned path has Windows 8.3 short paths resolved.
func CreateTempGitRepo(t *testing.T) string {
	t.Helper()
	SkipIfNoGit(t)

	dir := ResolvePath(t.TempDir())
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}
	// Create an initial commit so HEAD exists.
	dummyFile := filepath.Join(dir, "develop-README.md")
	if err := os.WriteFile(dummyFile, []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}
	return dir
}
