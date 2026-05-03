package sessionmemo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"myT-x/internal/sessioninfo"
)

func newTestService(t *testing.T) (*Service, string, string) {
	t.Helper()
	rootPath := filepath.Join(t.TempDir(), "workspace")
	service, configDir := newTestServiceWithRootFunc(t, func(string) string {
		return rootPath
	})
	return service, rootPath, configDir
}

func newTestServiceWithRootFunc(t *testing.T, rootForSession func(string) string) (*Service, string) {
	t.Helper()
	configDir := filepath.Join(t.TempDir(), "config")
	return NewService(Deps{
		ResolveSessionWorkDir: func(sessionName string) (string, error) {
			return rootForSession(sessionName), nil
		},
		ConfigDir: func() (string, error) {
			return configDir, nil
		},
	}), configDir
}

func memoPathForTest(t *testing.T, configDir, workDir string) string {
	t.Helper()
	path, err := sessioninfo.FilePath(configDir, workDir, memoFileName)
	if err != nil {
		t.Fatalf("session memo path: %v", err)
	}
	return path
}

func TestNewServicePanicsOnMissingDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing deps")
		}
	}()
	NewService(Deps{})
}

func TestLoadReturnsEmptyWhenMemoFileMissing(t *testing.T) {
	service, _, _ := newTestService(t)

	memo, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if memo != "" {
		t.Fatalf("Load() = %q, want empty memo", memo)
	}
}

func TestSaveAndLoadMemo(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	want := "\nKeep leading whitespace.\nAnd trailing whitespace.\n"

	if err := service.Save("alpha", want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	memoPath := memoPathForTest(t, configDir, rootPath)
	data, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", memoPath, err)
	}
	if string(data) != want {
		t.Fatalf("saved memo = %q, want %q", string(data), want)
	}

	got, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %q, want %q", got, want)
	}
}

func TestSaveReplacesExistingMemoFile(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	if err := service.Save("alpha", "first memo"); err != nil {
		t.Fatalf("Save() first error = %v", err)
	}
	if err := service.Save("alpha", "second memo"); err != nil {
		t.Fatalf("Save() second error = %v", err)
	}

	memoPath := memoPathForTest(t, configDir, rootPath)
	data, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", memoPath, err)
	}
	if string(data) != "second memo" {
		t.Fatalf("saved memo = %q, want replacement content", string(data))
	}
}

func TestLoadReadsMemoFileAfterExternalChange(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	if err := service.Save("alpha", "in memory"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	memoPath := memoPathForTest(t, configDir, rootPath)
	if err := os.WriteFile(memoPath, []byte("changed on disk"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "changed on disk" {
		t.Fatalf("Load() = %q, want memo reloaded from disk", got)
	}
}

func TestLoadCacheIsScopedByResolvedMemoPath(t *testing.T) {
	rootBySession := map[string]string{
		"alpha": filepath.Join(t.TempDir(), "workspace-one"),
	}
	service, configDir := newTestServiceWithRootFunc(t, func(sessionName string) string {
		return rootBySession[sessionName]
	})
	if err := service.Save("alpha", "old workspace memo"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	rootBySession["alpha"] = filepath.Join(t.TempDir(), "workspace-two")
	memoPath := memoPathForTest(t, configDir, rootBySession["alpha"])
	if err := os.MkdirAll(filepath.Dir(memoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(memoPath, []byte("new workspace memo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "new workspace memo" {
		t.Fatalf("Load() = %q, want memo from the new resolved path", got)
	}
}

func TestSaveRejectsEmptySessionName(t *testing.T) {
	service, _, _ := newTestService(t)

	err := service.Save("  ", "memo")
	if err == nil {
		t.Fatal("Save() expected session name error")
	}
	if !strings.Contains(err.Error(), "session name is required") {
		t.Fatalf("Save() error = %v, want session name error", err)
	}
}

func TestSaveRejectsOversizedMemo(t *testing.T) {
	service, _, _ := newTestService(t)

	err := service.Save("alpha", strings.Repeat("x", maxMemoBytes+1))
	if err == nil {
		t.Fatal("Save() expected size error")
	}
	if !strings.Contains(err.Error(), "bytes or fewer") {
		t.Fatalf("Save() error = %v, want size error", err)
	}
}

func TestSaveAcceptsMultibyteMemoAtByteLimit(t *testing.T) {
	service, _, _ := newTestService(t)
	memo := strings.Repeat("界", maxMemoBytes/3) + "x"
	if len([]byte(memo)) != maxMemoBytes {
		t.Fatalf("test memo byte length = %d, want %d", len([]byte(memo)), maxMemoBytes)
	}

	if err := service.Save("alpha", memo); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestSaveRejectsMultibyteMemoOverByteLimit(t *testing.T) {
	service, _, _ := newTestService(t)
	memo := strings.Repeat("界", maxMemoBytes/3) + "xx"
	if len([]byte(memo)) != maxMemoBytes+1 {
		t.Fatalf("test memo byte length = %d, want %d", len([]byte(memo)), maxMemoBytes+1)
	}

	err := service.Save("alpha", memo)
	if err == nil {
		t.Fatal("Save() expected size error")
	}
	if !strings.Contains(err.Error(), "bytes or fewer") {
		t.Fatalf("Save() error = %v, want size error", err)
	}
}

func TestLoadRejectsOversizedMemoFile(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	memoPath := memoPathForTest(t, configDir, rootPath)
	if err := os.MkdirAll(filepath.Dir(memoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(memoPath, []byte(strings.Repeat("x", maxMemoBytes+1)), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := service.Load("alpha")
	if err == nil {
		t.Fatal("Load() expected size error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("Load() error = %v, want size error", err)
	}
}

func TestCleanupSessionRemovesCachedMemo(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	if err := service.Save("alpha", "cached memo"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := service.CleanupSession("alpha"); err != nil {
		t.Fatalf("CleanupSession() error = %v", err)
	}

	memoPath := memoPathForTest(t, configDir, rootPath)
	if err := os.WriteFile(memoPath, []byte("disk memo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "disk memo" {
		t.Fatalf("Load() = %q, want memo from disk after cleanup", got)
	}
}

func TestRenameSessionMovesCachedMemoIndex(t *testing.T) {
	rootBySession := map[string]string{
		"alpha":         filepath.Join(t.TempDir(), "workspace"),
		"renamed-alpha": filepath.Join(t.TempDir(), "workspace-renamed"),
	}
	service, configDir := newTestServiceWithRootFunc(t, func(sessionName string) string {
		return rootBySession[sessionName]
	})
	if err := service.Save("alpha", "cached memo"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	rootBySession["renamed-alpha"] = rootBySession["alpha"]
	if err := service.RenameSession("alpha", "renamed-alpha"); err != nil {
		t.Fatalf("RenameSession() error = %v", err)
	}
	if err := service.CleanupSession("renamed-alpha"); err != nil {
		t.Fatalf("CleanupSession() error = %v", err)
	}

	memoPath := memoPathForTest(t, configDir, rootBySession["renamed-alpha"])
	if err := os.WriteFile(memoPath, []byte("renamed disk memo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := service.Load("renamed-alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "renamed disk memo" {
		t.Fatalf("Load() = %q, want memo from disk after renamed cleanup", got)
	}
}

func TestSaveCreatesSessionInfoDirectoryOnly(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	if _, err := os.Stat(filepath.Join(rootPath, ".myT-x")); !os.IsNotExist(err) {
		t.Fatalf("precondition: .myT-x should not exist: err=%v", err)
	}

	if err := service.Save("alpha", "memo"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootPath, ".myT-x")); !os.IsNotExist(err) {
		t.Errorf("Save() created workDir .myT-x, err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(memoPathForTest(t, configDir, rootPath))); err != nil {
		t.Errorf("session-info memo directory not created: %v", err)
	}
}

func TestLoadMigratesLegacyProjectMemo(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	legacyDir := filepath.Join(rootPath, ".myT-x")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, memoFileName), []byte("legacy memo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "legacy memo" {
		t.Fatalf("Load() = %q, want migrated legacy memo", got)
	}
	memoPath := memoPathForTest(t, configDir, rootPath)
	data, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", memoPath, err)
	}
	if string(data) != "legacy memo" {
		t.Fatalf("migrated memo = %q, want legacy memo", string(data))
	}
	if data, err := os.ReadFile(filepath.Join(legacyDir, memoFileName)); err != nil || string(data) != "legacy memo" {
		t.Fatalf("legacy memo should remain untouched, data=%q err=%v", string(data), err)
	}
}

func TestLoadPrefersSessionInfoMemoOverLegacyProjectMemo(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	legacyDir := filepath.Join(rootPath, ".myT-x")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() legacy error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, memoFileName), []byte("legacy memo"), 0o644); err != nil {
		t.Fatalf("WriteFile() legacy error = %v", err)
	}
	memoPath := memoPathForTest(t, configDir, rootPath)
	if err := os.MkdirAll(filepath.Dir(memoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() current error = %v", err)
	}
	if err := os.WriteFile(memoPath, []byte("current memo"), 0o644); err != nil {
		t.Fatalf("WriteFile() current error = %v", err)
	}

	got, err := service.Load("alpha")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != "current memo" {
		t.Fatalf("Load() = %q, want current session-info memo", got)
	}
}

func TestSaveRemovesTempFileWhenReplaceFails(t *testing.T) {
	service, rootPath, configDir := newTestService(t)
	memoPath := memoPathForTest(t, configDir, rootPath)
	if err := os.MkdirAll(memoPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", memoPath, err)
	}

	err := service.Save("alpha", "memo")
	if err == nil {
		t.Fatal("Save() expected replace error")
	}

	entries, readErr := os.ReadDir(filepath.Dir(memoPath))
	if readErr != nil {
		t.Fatalf("ReadDir() error = %v", readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".session-memo.") {
			t.Fatalf("temporary memo file was not removed: %s", entry.Name())
		}
	}
}

func TestConcurrentSaveAndLoad(t *testing.T) {
	service, _, _ := newTestService(t)
	if err := service.Save("alpha", "seed"); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := service.Save("alpha", fmt.Sprintf("memo-%02d", i)); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := service.Load("alpha"); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Save/Load error = %v", err)
	}
}
