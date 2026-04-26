package testutil

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	localGitTransportOnce sync.Once
	localGitTransportErr  error
)

// SkipIfNoGit skips the test if git is not available.
func SkipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping")
	}
}

// SkipIfNoLocalGitTransport skips tests that require local clone/push transport.
func SkipIfNoLocalGitTransport(t *testing.T) {
	t.Helper()
	SkipIfNoGit(t)

	localGitTransportOnce.Do(func() {
		localGitTransportErr = checkLocalGitTransport()
	})
	if localGitTransportErr != nil {
		t.Skipf("local git transport unavailable: %v", localGitTransportErr)
	}
}

func checkLocalGitTransport() error {
	root, err := os.MkdirTemp("", "mytx-git-transport-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(root)

	bareDir := filepath.Join(root, "origin.git")
	workDir := filepath.Join(root, "work")
	cloneDir := filepath.Join(root, "clone")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		return fmt.Errorf("create clone dir: %w", err)
	}

	if out, err := runGitForTransportCheck(root, "init", "--bare", bareDir); err != nil {
		return fmt.Errorf("git init --bare: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(workDir, "init"); err != nil {
		return fmt.Errorf("git init work tree: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(workDir, "config", "user.email", "test@test.com"); err != nil {
		return fmt.Errorf("git config user.email: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(workDir, "config", "user.name", "Test"); err != nil {
		return fmt.Errorf("git config user.name: %w: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# test"), 0o644); err != nil {
		return fmt.Errorf("write seed file: %w", err)
	}
	if out, err := runGitForTransportCheck(workDir, "add", "."); err != nil {
		return fmt.Errorf("git add: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(workDir, "commit", "-m", "initial"); err != nil {
		return fmt.Errorf("git commit: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(workDir, "remote", "add", "origin", bareDir); err != nil {
		return fmt.Errorf("git remote add: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(workDir, "push", "-u", "origin", "HEAD"); err != nil {
		return fmt.Errorf("git push local remote: %w: %s", err, out)
	}
	if out, err := runGitForTransportCheck(cloneDir, "clone", bareDir, "."); err != nil {
		return fmt.Errorf("git clone local remote: %w: %s", err, out)
	}
	return nil
}

func runGitForTransportCheck(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, os.Environ()...), "LC_ALL=C", "LC_MESSAGES=C", "LANG=C")
	out, err := cmd.CombinedOutput()
	return string(out), err
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
