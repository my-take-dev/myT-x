package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/scheduler"
	"myT-x/internal/sessioninfo"
	"myT-x/internal/tmux"
	"myT-x/internal/usagedashboard"
)

func setupUninitializedConfigPathSessionApp(t *testing.T) (*App, string) {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", "")

	app := NewApp()
	if got := app.configState.ConfigPath(); got != "" {
		t.Fatalf("ConfigPath() before Initialize = %q, want empty", got)
	}

	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("test-session", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	workDir := t.TempDir()
	if err := app.sessions.SetRootPath("test-session", workDir); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}
	return app, workDir
}

func assertDefaultSessionInfoNotCreated(t *testing.T) {
	t.Helper()
	defaultSessionInfoDir := filepath.Join(filepath.Dir(config.DefaultPath()), sessioninfo.DirName)
	if _, err := os.Stat(defaultSessionInfoDir); !os.IsNotExist(err) {
		t.Fatalf("default session-info directory should not be created when ConfigPath is uninitialized: %v", err)
	}
}

func TestSaveSessionMemoRejectsUninitializedConfigPath(t *testing.T) {
	app, _ := setupUninitializedConfigPathSessionApp(t)

	err := app.SaveSessionMemo("test-session", "memo")
	if err == nil {
		t.Fatal("SaveSessionMemo() expected uninitialized config path error")
	}
	if !strings.Contains(err.Error(), "config path is not initialized") {
		t.Fatalf("SaveSessionMemo() error = %v, want uninitialized config path error", err)
	}
	assertDefaultSessionInfoNotCreated(t)
}

func TestSaveSchedulerTemplateRejectsUninitializedConfigPath(t *testing.T) {
	app, _ := setupUninitializedConfigPathSessionApp(t)

	err := app.SaveSchedulerTemplate("test-session", scheduler.Template{
		Title:           "Template",
		Message:         "message",
		IntervalSeconds: 10,
		MaxCount:        1,
	})
	if err == nil {
		t.Fatal("SaveSchedulerTemplate() expected uninitialized config path error")
	}
	if !strings.Contains(err.Error(), "config path is not initialized") {
		t.Fatalf("SaveSchedulerTemplate() error = %v, want uninitialized config path error", err)
	}
	assertDefaultSessionInfoNotCreated(t)
}

func TestUsageDashboardDoesNotFallbackToDefaultConfigPathWhenUninitialized(t *testing.T) {
	app, _ := setupUninitializedConfigPathSessionApp(t)
	homeDir := t.TempDir()
	app.usageDashboard = usagedashboard.NewService(usagedashboard.Deps{
		ResolveSessionWorkDir: app.sessionService.ResolveSessionWorkDir,
		HomeDir:               func() (string, error) { return homeDir, nil },
		ConfigDir:             appConfigDirProvider(app),
		NowFunc: func() time.Time {
			return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
		},
	})

	snapshot, err := app.GetUsageDashboard("test-session", "both", true)
	if err != nil {
		t.Fatalf("GetUsageDashboard() should still return fresh data when cache persistence is unavailable: %v", err)
	}
	if snapshot.LastUpdatedAt.IsZero() {
		t.Fatal("GetUsageDashboard() returned zero LastUpdatedAt")
	}
	assertDefaultSessionInfoNotCreated(t)
}
