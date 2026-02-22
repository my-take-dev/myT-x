package config

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func newConfigPathForSaveTest(t *testing.T, elems ...string) string {
	t.Helper()
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	defaultPath := DefaultPath()

	return filepath.Join(filepath.Dir(defaultPath), filepath.Join(elems...))
}

func TestPathWithinDir(t *testing.T) {
	baseDir := t.TempDir()
	configDir := filepath.Join(baseDir, "config")

	tests := []struct {
		name string
		path string
		dir  string
		want bool
	}{
		{
			name: "same path",
			path: configDir,
			dir:  configDir,
			want: true,
		},
		{
			name: "subdirectory path",
			path: filepath.Join(configDir, "sub", "config.yaml"),
			dir:  configDir,
			want: true,
		},
		{
			name: "traversal path",
			path: filepath.Join(configDir, "..", "outside.yaml"),
			dir:  configDir,
			want: false,
		},
		{
			name: "different path",
			path: filepath.Join(baseDir, "other", "config.yaml"),
			dir:  configDir,
			want: false,
		},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			path string
			dir  string
			want bool
		}{
			name: "different drive",
			path: `D:\outside\config.yaml`,
			dir:  `C:\inside`,
			want: false,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathWithinDir(tt.path, tt.dir)
			if got != tt.want {
				t.Fatalf("pathWithinDir(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}

func TestIsZeroConfig(t *testing.T) {
	t.Run("empty config is zero", func(t *testing.T) {
		if !isZeroConfig(Config{}) {
			t.Fatal("isZeroConfig(Config{}) = false, want true")
		}
	})

	t.Run("default config is not zero", func(t *testing.T) {
		if isZeroConfig(DefaultConfig()) {
			t.Fatal("isZeroConfig(DefaultConfig()) = true, want false")
		}
	})

	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "shell set",
			mutate: func(cfg *Config) {
				cfg.Shell = "pwsh.exe"
			},
		},
		{
			name: "prefix set",
			mutate: func(cfg *Config) {
				cfg.Prefix = "Ctrl+b"
			},
		},
		{
			name: "keys map set",
			mutate: func(cfg *Config) {
				cfg.Keys = map[string]string{"k": "v"}
			},
		},
		{
			name: "keys map non-nil empty",
			mutate: func(cfg *Config) {
				cfg.Keys = map[string]string{}
			},
		},
		{
			name: "quake mode enabled",
			mutate: func(cfg *Config) {
				cfg.QuakeMode = true
			},
		},
		{
			name: "global hotkey set",
			mutate: func(cfg *Config) {
				cfg.GlobalHotkey = "Ctrl+Shift+F12"
			},
		},
		{
			name: "worktree enabled",
			mutate: func(cfg *Config) {
				cfg.Worktree.Enabled = true
			},
		},
		{
			name: "worktree force cleanup enabled",
			mutate: func(cfg *Config) {
				cfg.Worktree.ForceCleanup = true
			},
		},
		{
			name: "worktree setup scripts non-nil empty",
			mutate: func(cfg *Config) {
				cfg.Worktree.SetupScripts = make([]string, 0)
			},
		},
		{
			name: "worktree copy files non-nil empty",
			mutate: func(cfg *Config) {
				cfg.Worktree.CopyFiles = make([]string, 0)
			},
		},
		{
			name: "worktree copy dirs non-nil empty",
			mutate: func(cfg *Config) {
				cfg.Worktree.CopyDirs = make([]string, 0)
			},
		},
		{
			name: "worktree copy dirs set",
			mutate: func(cfg *Config) {
				cfg.Worktree.CopyDirs = []string{".vscode"}
			},
		},
		{
			name: "agent model set",
			mutate: func(cfg *Config) {
				cfg.AgentModel = &AgentModel{}
			},
		},
		{
			name: "pane env set",
			mutate: func(cfg *Config) {
				cfg.PaneEnv = map[string]string{"K": "V"}
			},
		},
		{
			name: "pane env default enabled",
			mutate: func(cfg *Config) {
				cfg.PaneEnvDefaultEnabled = true
			},
		},
		{
			name: "claude env set",
			mutate: func(cfg *Config) {
				cfg.ClaudeEnv = &ClaudeEnvConfig{}
			},
		},
		{
			name: "websocket port set",
			mutate: func(cfg *Config) {
				cfg.WebSocketPort = 8080
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{}
			tt.mutate(&cfg)
			if isZeroConfig(cfg) {
				t.Fatal("isZeroConfig() = true, want false")
			}
		})
	}
}

func TestLoadRejectsShellOutsideAllowlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("shell: C:\\\\malicious\\\\evil.exe\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("Load() expected allowlist validation error")
	}
}

func TestLoadAcceptsAllowlistedShellName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("shell: cmd.exe\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Shell != "cmd.exe" {
		t.Fatalf("cfg.Shell = %q, want cmd.exe", cfg.Shell)
	}
}

func TestDefaultPathUsesLocalAppDataWhenAvailable(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\tester\AppData\Local`)
	t.Setenv("APPDATA", "")

	path := DefaultPath()

	want := filepath.Join(`C:\Users\tester\AppData\Local`, "myT-x", "config.yaml")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestLoadRejectsShellWithNullByte(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	// Write an actual null byte inside the shell value.
	raw := []byte("shell: \"cmd.exe\x00.evil\"\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("Load() expected null byte validation error")
	}
}

func TestLoadRejectsRelativePathShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("shell: .\\tools\\cmd.exe\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("Load() expected relative path validation error")
	}
}

func TestLoadAcceptsCaseInsensitiveShellName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("shell: CMD.EXE\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Shell != "CMD.EXE" {
		t.Fatalf("cfg.Shell = %q, want CMD.EXE", cfg.Shell)
	}
}

func TestDefaultPathFallsBackToAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("APPDATA", `C:\Users\tester\AppData\Roaming`)

	path := DefaultPath()

	want := filepath.Join(`C:\Users\tester\AppData\Roaming`, "myT-x", "config.yaml")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestDefaultPathFallsBackToTempDirWhenHomeDirUnavailable(t *testing.T) {
	originalUserHomeDirFn := userHomeDirFn
	t.Cleanup(func() {
		userHomeDirFn = originalUserHomeDirFn
	})
	ConsumeDefaultPathWarnings()
	t.Cleanup(func() {
		ConsumeDefaultPathWarnings()
	})

	userHomeDirFn = func() (string, error) {
		return "", errors.New("simulated home dir resolution failure")
	}
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("APPDATA", "")

	path := DefaultPath()
	want := filepath.Join(os.TempDir(), "myT-x", "config.yaml")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestDefaultPathLogsWarningWhenFallingBackToTempDir(t *testing.T) {
	originalUserHomeDirFn := userHomeDirFn
	originalLogger := slog.Default()
	t.Cleanup(func() {
		userHomeDirFn = originalUserHomeDirFn
		slog.SetDefault(originalLogger)
	})
	ConsumeDefaultPathWarnings()
	t.Cleanup(func() {
		ConsumeDefaultPathWarnings()
	})

	var logBuf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	userHomeDirFn = func() (string, error) {
		return "", errors.New("simulated home dir resolution failure")
	}
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("APPDATA", "")

	_ = DefaultPath()

	if !strings.Contains(logBuf.String(), "using temp dir as config path fallback") {
		t.Fatalf("log output = %q, want temp-dir fallback warning", logBuf.String())
	}
}

func TestDefaultPathRecordsUserVisibleWarningOnTempDirFallback(t *testing.T) {
	originalUserHomeDirFn := userHomeDirFn
	t.Cleanup(func() {
		userHomeDirFn = originalUserHomeDirFn
	})
	ConsumeDefaultPathWarnings()
	t.Cleanup(func() {
		ConsumeDefaultPathWarnings()
	})

	userHomeDirFn = func() (string, error) {
		return "", errors.New("simulated home dir resolution failure")
	}
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("APPDATA", "")

	_ = DefaultPath()
	warnings := ConsumeDefaultPathWarnings()
	if len(warnings) == 0 {
		t.Fatal("ConsumeDefaultPathWarnings() returned no warning for temp-dir fallback")
	}
	if !strings.Contains(warnings[0], "Config path fallback") {
		t.Fatalf("warning = %q, want fallback message", warnings[0])
	}
}

func TestLoadRejectsForwardSlashRelativePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("shell: subdir/cmd.exe\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("Load() expected relative path validation error for forward slash")
	}
}

func TestLoadRejectsAbsolutePathThatDoesNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if runtime.GOOS == "windows" {
		raw := []byte("shell: C:\\nonexistent\\cmd.exe\n")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	} else {
		raw := []byte("shell: /nonexistent/cmd.exe\n")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("Load() expected error for non-existent absolute path")
	}
}

func TestLoadRejectsAbsolutePathThatIsDirectory(t *testing.T) {
	dir := t.TempDir()
	// Create a directory named "cmd.exe" to test the is-directory check.
	dirShell := filepath.Join(dir, "cmd.exe")
	if err := os.MkdirAll(dirShell, 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}

	configPath := filepath.Join(dir, "config.yaml")
	raw := []byte("shell: " + dirShell + "\n")
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(configPath); err == nil {
		t.Fatalf("Load() expected error for directory shell path")
	}
}

func TestLoadIgnoresUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte(`
worktree:
  enabled: true
  auto_cleanup: true
  force_cleanup: false
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() should accept configs with removed fields: %v", err)
	}
	if !cfg.Worktree.Enabled {
		t.Error("Worktree.Enabled should be true")
	}
}

func TestLoadWorktreeForceCleanup(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool
	}{
		{"force_cleanup true", "worktree:\n  force_cleanup: true\n", true},
		{"force_cleanup false", "worktree:\n  force_cleanup: false\n", false},
		{"force_cleanup omitted", "worktree:\n  enabled: true\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Worktree.ForceCleanup != tt.want {
				t.Errorf("ForceCleanup = %v, want %v", cfg.Worktree.ForceCleanup, tt.want)
			}
		})
	}
}

func TestLoadWorktreeEnabledDefaultAppliedWhenEnabledFieldMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("worktree:\n  copy_files:\n    - .env\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Worktree.Enabled {
		t.Fatal("Worktree.Enabled should default to true when enabled is omitted")
	}
}

func TestLoadWorktreeEnabledExplicitFalsePreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("worktree:\n  enabled: false\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Worktree.Enabled {
		t.Fatal("Worktree.Enabled should remain false when explicitly configured")
	}
}

func TestDefaultConfigWorktreeDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Worktree.Enabled {
		t.Error("Worktree.Enabled default should be true")
	}
	if cfg.Worktree.ForceCleanup {
		t.Error("Worktree.ForceCleanup default should be false")
	}
	if cfg.Worktree.SetupScripts == nil || len(cfg.Worktree.SetupScripts) != 0 {
		t.Errorf("Worktree.SetupScripts: want non-nil empty slice, got %v", cfg.Worktree.SetupScripts)
	}
	if cfg.Worktree.CopyFiles == nil || len(cfg.Worktree.CopyFiles) != 0 {
		t.Errorf("Worktree.CopyFiles: want non-nil empty slice, got %v", cfg.Worktree.CopyFiles)
	}
	if cfg.Worktree.CopyDirs == nil || len(cfg.Worktree.CopyDirs) != 0 {
		t.Errorf("Worktree.CopyDirs: want non-nil empty slice, got %v", cfg.Worktree.CopyDirs)
	}
}

func TestLoadAgentModel(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		wantNil  bool
		wantFrom string
		wantTo   string
	}{
		{
			name:     "both from and to set",
			yaml:     "agent_model:\n  from: claude-opus-4-6\n  to: claude-sonnet-4-5\n",
			wantNil:  false,
			wantFrom: "claude-opus-4-6",
			wantTo:   "claude-sonnet-4-5",
		},
		{
			name:     "from and to are trimmed",
			yaml:     "agent_model:\n  from: \"  claude-opus-4-6  \"\n  to: \"  claude-sonnet-4-5  \"\n",
			wantNil:  false,
			wantFrom: "claude-opus-4-6",
			wantTo:   "claude-sonnet-4-5",
		},
		{
			name:    "agent_model omitted",
			yaml:    "shell: powershell.exe\n",
			wantNil: true,
		},
		{
			name:    "from only",
			yaml:    "agent_model:\n  from: claude-opus-4-6\n",
			wantErr: true,
		},
		{
			name:    "to only",
			yaml:    "agent_model:\n  to: claude-sonnet-4-5\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if tt.wantNil {
				if cfg.AgentModel != nil {
					t.Errorf("AgentModel should be nil, got %+v", cfg.AgentModel)
				}
				return
			}
			if cfg.AgentModel == nil {
				t.Fatal("AgentModel is nil")
			}
			if cfg.AgentModel.From != tt.wantFrom {
				t.Errorf("From = %q, want %q", cfg.AgentModel.From, tt.wantFrom)
			}
			if cfg.AgentModel.To != tt.wantTo {
				t.Errorf("To = %q, want %q", cfg.AgentModel.To, tt.wantTo)
			}
		})
	}
}

func TestDefaultConfigAgentModelNil(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.AgentModel != nil {
		t.Errorf("DefaultConfig().AgentModel should be nil, got %+v", cfg.AgentModel)
	}
}

func TestLoadAgentModelOverrides(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantErr    bool
		wantCount  int
		wantNames  []string
		wantModels []string
	}{
		{
			name: "valid overrides",
			yaml: `agent_model:
  from: claude-opus-4-6
  to: claude-sonnet-4-5
  overrides:
    - name: security
      model: claude-opus-4-6
    - name: reviewer
      model: claude-sonnet-4-5
`,
			wantCount:  2,
			wantNames:  []string{"security", "reviewer"},
			wantModels: []string{"claude-opus-4-6", "claude-sonnet-4-5"},
		},
		{
			name: "name too short 4 chars",
			yaml: `agent_model:
  overrides:
    - name: test
      model: claude-opus-4-6
`,
			wantErr: true,
		},
		{
			name: "name exactly 5 chars",
			yaml: `agent_model:
  overrides:
    - name: coder
      model: claude-opus-4-6
`,
			wantCount:  1,
			wantNames:  []string{"coder"},
			wantModels: []string{"claude-opus-4-6"},
		},
		{
			name: "empty model",
			yaml: `agent_model:
  overrides:
    - name: security
      model: ""
`,
			wantErr: true,
		},
		{
			name: "empty overrides list",
			yaml: `agent_model:
  from: claude-opus-4-6
  to: claude-sonnet-4-5
  overrides: []
`,
			wantCount: 0,
		},
		{
			name: "overrides without from/to",
			yaml: `agent_model:
  overrides:
    - name: security
      model: claude-opus-4-6
`,
			wantCount:  1,
			wantNames:  []string{"security"},
			wantModels: []string{"claude-opus-4-6"},
		},
		{
			name: "whitespace-only name",
			yaml: `agent_model:
  overrides:
    - name: "   "
      model: claude-opus-4-6
`,
			wantErr: true,
		},
		{
			name: "whitespace-padded name trimmed",
			yaml: `agent_model:
  overrides:
    - name: "  security  "
      model: claude-opus-4-6
`,
			wantCount:  1,
			wantNames:  []string{"security"},
			wantModels: []string{"claude-opus-4-6"},
		},
		{
			name: "multiple overrides preserve order",
			yaml: `agent_model:
  overrides:
    - name: security
      model: claude-opus-4-6
    - name: reviewer
      model: claude-sonnet-4-5
    - name: coder1
      model: claude-haiku-4
`,
			wantCount:  3,
			wantNames:  []string{"security", "reviewer", "coder1"},
			wantModels: []string{"claude-opus-4-6", "claude-sonnet-4-5", "claude-haiku-4"},
		},
		{
			name: "duplicate override names are allowed",
			yaml: `agent_model:
  overrides:
    - name: security
      model: claude-opus-4-6
    - name: security
      model: claude-sonnet-4-5
`,
			wantCount:  2,
			wantNames:  []string{"security", "security"},
			wantModels: []string{"claude-opus-4-6", "claude-sonnet-4-5"},
		},
		{
			name: "override name with regex metacharacters is preserved",
			yaml: `agent_model:
  overrides:
    - name: "sec.*rity"
      model: claude-opus-4-6
`,
			wantCount:  1,
			wantNames:  []string{"sec.*rity"},
			wantModels: []string{"claude-opus-4-6"},
		},
		{
			name: "non ascii name with enough runes",
			yaml: `agent_model:
  overrides:
    - name: "\u30bb\u30ad\u30e5\u30ea\u30c6\u30a3\u62c5\u5f53"
      model: claude-opus-4-6
`,
			wantCount:  1,
			wantNames:  []string{"\u30bb\u30ad\u30e5\u30ea\u30c6\u30a3\u62c5\u5f53"},
			wantModels: []string{"claude-opus-4-6"},
		},
		{
			name:       "very long name is accepted",
			yaml:       "agent_model:\n  overrides:\n    - name: \"" + strings.Repeat("a", 512) + "\"\n      model: claude-opus-4-6\n",
			wantCount:  1,
			wantNames:  []string{strings.Repeat("a", 512)},
			wantModels: []string{"claude-opus-4-6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.AgentModel == nil {
				t.Fatal("AgentModel is nil")
			}
			if got := len(cfg.AgentModel.Overrides); got != tt.wantCount {
				t.Fatalf("Overrides count = %d, want %d", got, tt.wantCount)
			}
			for i, name := range tt.wantNames {
				if cfg.AgentModel.Overrides[i].Name != name {
					t.Errorf("Overrides[%d].Name = %q, want %q", i, cfg.AgentModel.Overrides[i].Name, name)
				}
			}
			for i, model := range tt.wantModels {
				if cfg.AgentModel.Overrides[i].Model != model {
					t.Errorf("Overrides[%d].Model = %q, want %q", i, cfg.AgentModel.Overrides[i].Model, model)
				}
			}
		})
	}
}

func TestAgentModelStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[AgentModelOverride]().NumField(); got != 2 {
		t.Fatalf("AgentModelOverride field count = %d, want 2", got)
	}
	if got := reflect.TypeFor[AgentModel]().NumField(); got != 3 {
		t.Fatalf("AgentModel field count = %d, want 3", got)
	}
}

func TestNormalizeAndValidateAgentModel(t *testing.T) {
	t.Run("nil model is valid", func(t *testing.T) {
		if err := normalizeAndValidateAgentModel(nil); err != nil {
			t.Fatalf("normalizeAndValidateAgentModel(nil) error = %v", err)
		}
	})

	t.Run("trims from to and overrides", func(t *testing.T) {
		am := &AgentModel{
			From: "  claude-opus-4-6  ",
			To:   "  claude-sonnet-4-5  ",
			Overrides: []AgentModelOverride{
				{Name: "  security  ", Model: "  claude-opus-4-6  "},
			},
		}
		if err := normalizeAndValidateAgentModel(am); err != nil {
			t.Fatalf("normalizeAndValidateAgentModel() error = %v", err)
		}
		if am.From != "claude-opus-4-6" {
			t.Fatalf("From = %q, want %q", am.From, "claude-opus-4-6")
		}
		if am.To != "claude-sonnet-4-5" {
			t.Fatalf("To = %q, want %q", am.To, "claude-sonnet-4-5")
		}
		if am.Overrides[0].Name != "security" {
			t.Fatalf("Overrides[0].Name = %q, want %q", am.Overrides[0].Name, "security")
		}
		if am.Overrides[0].Model != "claude-opus-4-6" {
			t.Fatalf("Overrides[0].Model = %q, want %q", am.Overrides[0].Model, "claude-opus-4-6")
		}
	})

	t.Run("from without to is rejected", func(t *testing.T) {
		am := &AgentModel{From: "claude-opus-4-6"}
		if err := normalizeAndValidateAgentModel(am); err == nil {
			t.Fatal("expected error for from without to")
		}
	})

	t.Run("to without from is rejected", func(t *testing.T) {
		am := &AgentModel{To: "claude-sonnet-4-5"}
		if err := normalizeAndValidateAgentModel(am); err == nil {
			t.Fatal("expected error for to without from")
		}
	})

	// ALL wildcard semantics: when From is "ALL" (case-insensitive), every
	// --model value in child agent commands is replaced with To, regardless of
	// the current model name. This is a blanket substitution mode. The actual
	// matching logic lives in isAllModelFrom() in tmux-shim/model_transform.go;
	// the config layer only validates that "ALL" is accepted as a valid From value.
	t.Run("ALL wildcard from is accepted", func(t *testing.T) {
		am := &AgentModel{From: "ALL", To: "claude-sonnet-4-5"}
		if err := normalizeAndValidateAgentModel(am); err != nil {
			t.Fatalf("normalizeAndValidateAgentModel(ALL) error = %v", err)
		}
		if am.From != "ALL" {
			t.Fatalf("From = %q, want %q", am.From, "ALL")
		}
	})

	t.Run("ALL wildcard from with whitespace is trimmed", func(t *testing.T) {
		am := &AgentModel{From: "  ALL  ", To: "claude-sonnet-4-5"}
		if err := normalizeAndValidateAgentModel(am); err != nil {
			t.Fatalf("normalizeAndValidateAgentModel(ALL trimmed) error = %v", err)
		}
		if am.From != "ALL" {
			t.Fatalf("From = %q, want %q", am.From, "ALL")
		}
	})

	t.Run("short unicode name is rejected by rune count", func(t *testing.T) {
		am := &AgentModel{
			Overrides: []AgentModelOverride{
				{Name: "\u5b89\u5168", Model: "claude-opus-4-6"},
			},
		}
		if err := normalizeAndValidateAgentModel(am); err == nil {
			t.Fatal("expected error for short override name")
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "sub", "config.yaml")
		cfg := DefaultConfig()
		if _, err := Save(path, cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat config: %v", err)
		}
		if info.IsDir() {
			t.Fatal("Save() created a directory instead of file")
		}
	})

	t.Run("round trip", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := DefaultConfig()
		cfg.QuakeMode = false
		cfg.GlobalHotkey = "Ctrl+Alt+T"
		cfg.Keys = map[string]string{
			"split-vertical":   "%",
			"split-horizontal": "\"",
			"toggle-zoom":      "z",
			"kill-pane":        "x",
			"detach-session":   "d",
			"custom-action":    "c",
		}
		cfg.AgentModel = &AgentModel{
			From: "claude-opus-4-6",
			To:   "claude-sonnet-4-5",
			Overrides: []AgentModelOverride{
				{Name: "security", Model: "claude-opus-4-6"},
			},
		}
		cfg.Worktree.SetupScripts = []string{"npm install"}
		cfg.Worktree.CopyFiles = []string{".env"}
		cfg.Worktree.CopyDirs = []string{".vscode"}
		cfg.PaneEnv = map[string]string{"MY_VAR": "val", "ANOTHER": "x"}

		if _, err := Save(path, cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.Shell != cfg.Shell {
			t.Errorf("Shell = %q, want %q", loaded.Shell, cfg.Shell)
		}
		if loaded.Prefix != cfg.Prefix {
			t.Errorf("Prefix = %q, want %q", loaded.Prefix, cfg.Prefix)
		}
		if loaded.QuakeMode != cfg.QuakeMode {
			t.Errorf("QuakeMode = %v, want %v", loaded.QuakeMode, cfg.QuakeMode)
		}
		if loaded.GlobalHotkey != cfg.GlobalHotkey {
			t.Errorf("GlobalHotkey = %q, want %q", loaded.GlobalHotkey, cfg.GlobalHotkey)
		}
		if !reflect.DeepEqual(loaded.Keys, cfg.Keys) {
			t.Errorf("Keys = %v, want %v", loaded.Keys, cfg.Keys)
		}
		if loaded.AgentModel == nil {
			t.Fatal("AgentModel is nil after round-trip")
		}
		if loaded.AgentModel.From != "claude-opus-4-6" {
			t.Errorf("From = %q", loaded.AgentModel.From)
		}
		if len(loaded.AgentModel.Overrides) != 1 {
			t.Errorf("Overrides count = %d", len(loaded.AgentModel.Overrides))
		}
		if len(loaded.Worktree.SetupScripts) != 1 || loaded.Worktree.SetupScripts[0] != "npm install" {
			t.Errorf("SetupScripts = %v", loaded.Worktree.SetupScripts)
		}
		if len(loaded.Worktree.CopyFiles) != 1 || loaded.Worktree.CopyFiles[0] != ".env" {
			t.Errorf("CopyFiles = %v", loaded.Worktree.CopyFiles)
		}
		if len(loaded.Worktree.CopyDirs) != 1 || loaded.Worktree.CopyDirs[0] != ".vscode" {
			t.Errorf("CopyDirs = %v", loaded.Worktree.CopyDirs)
		}
		if !reflect.DeepEqual(loaded.PaneEnv, cfg.PaneEnv) {
			t.Errorf("PaneEnv = %v, want %v", loaded.PaneEnv, cfg.PaneEnv)
		}
	})

	t.Run("returns normalized config", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := Config{} // empty: defaults should be filled
		normalized, err := Save(path, cfg)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if normalized.Shell != "powershell.exe" {
			t.Errorf("normalized.Shell = %q, want powershell.exe", normalized.Shell)
		}
		if normalized.Prefix != "Ctrl+b" {
			t.Errorf("normalized.Prefix = %q, want Ctrl+b", normalized.Prefix)
		}
		if normalized.GlobalHotkey != DefaultConfig().GlobalHotkey {
			t.Errorf("normalized.GlobalHotkey = %q, want %q", normalized.GlobalHotkey, DefaultConfig().GlobalHotkey)
		}
		if normalized.Keys == nil {
			t.Error("normalized.Keys should not be nil")
		}
		if normalized.QuakeMode != DefaultConfig().QuakeMode {
			t.Errorf("normalized.QuakeMode = %v, want %v", normalized.QuakeMode, DefaultConfig().QuakeMode)
		}
		if normalized.Worktree.Enabled != DefaultConfig().Worktree.Enabled {
			t.Errorf("normalized.Worktree.Enabled = %v, want %v", normalized.Worktree.Enabled, DefaultConfig().Worktree.Enabled)
		}
		if normalized.Worktree.CopyDirs == nil || len(normalized.Worktree.CopyDirs) != 0 {
			t.Errorf("normalized.Worktree.CopyDirs = %v, want non-nil empty slice", normalized.Worktree.CopyDirs)
		}
	})

	t.Run("rejects invalid shell", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := DefaultConfig()
		cfg.Shell = "evil.exe"
		if _, err := Save(path, cfg); err == nil {
			t.Fatal("Save() expected shell validation error")
		}
	})

	t.Run("rejects shell with null byte", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := DefaultConfig()
		cfg.Shell = "cmd.exe\x00.evil"
		if _, err := Save(path, cfg); err == nil {
			t.Fatal("Save() expected null byte validation error")
		}
	})

	t.Run("rejects invalid agent model", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := DefaultConfig()
		cfg.AgentModel = &AgentModel{From: "only-from"}
		if _, err := Save(path, cfg); err == nil {
			t.Fatal("Save() expected agent model validation error")
		}
	})

	t.Run("rejects empty path", func(t *testing.T) {
		cfg := DefaultConfig()
		if _, err := Save("", cfg); err == nil {
			t.Fatal("Save() expected empty path error")
		}
	})

	t.Run("rejects whitespace-only path", func(t *testing.T) {
		cfg := DefaultConfig()
		if _, err := Save("   ", cfg); err == nil {
			t.Fatal("Save() expected whitespace-only path error")
		}
	})

	t.Run("fills defaults for empty shell and prefix", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := Config{}
		if _, err := Save(path, cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.Shell != "powershell.exe" {
			t.Errorf("Shell = %q, want powershell.exe", loaded.Shell)
		}
		if loaded.Prefix != "Ctrl+b" {
			t.Errorf("Prefix = %q, want Ctrl+b", loaded.Prefix)
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")

		cfg1 := DefaultConfig()
		cfg1.Shell = "cmd.exe"
		if _, err := Save(path, cfg1); err != nil {
			t.Fatalf("Save() initial error = %v", err)
		}

		cfg2 := DefaultConfig()
		cfg2.Shell = "pwsh.exe"
		cfg2.Prefix = "Ctrl+a"
		if _, err := Save(path, cfg2); err != nil {
			t.Fatalf("Save() overwrite error = %v", err)
		}

		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.Shell != "pwsh.exe" {
			t.Errorf("Shell = %q, want pwsh.exe (overwrite failed)", loaded.Shell)
		}
		if loaded.Prefix != "Ctrl+a" {
			t.Errorf("Prefix = %q, want Ctrl+a", loaded.Prefix)
		}
	})

	t.Run("rejects path outside default config directory", func(t *testing.T) {
		_ = newConfigPathForSaveTest(t, "config.yaml")
		outsidePath := filepath.Join(t.TempDir(), "outside-config.yaml")

		if _, err := Save(outsidePath, DefaultConfig()); err == nil {
			t.Fatal("Save() expected path validation error")
		}
	})

	t.Run("rename failure removes temp file", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("mkdir path as directory: %v", err)
		}

		if _, err := Save(path, DefaultConfig()); err == nil {
			t.Fatal("Save() expected rename failure")
		}

		pattern := filepath.Join(filepath.Dir(path), ".config.yaml.tmp.*")
		tempFiles, globErr := filepath.Glob(pattern)
		if globErr != nil {
			t.Fatalf("glob temp files: %v", globErr)
		}
		if len(tempFiles) != 0 {
			t.Fatalf("temporary files were not cleaned up: %v", tempFiles)
		}
	})
}

func TestReadLimitedFileRejectsTooLargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large-config.yaml")
	oversized := bytes.Repeat([]byte("a"), int(maxConfigFileBytes+1))
	if err := os.WriteFile(path, oversized, 0o600); err != nil {
		t.Fatalf("write oversized config: %v", err)
	}

	if _, err := readLimitedFile(path, maxConfigFileBytes); err == nil {
		t.Fatal("readLimitedFile() expected size limit error")
	}
}

func TestReadLimitedFileAllowsFileAtExactMaxBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exact-config.yaml")
	exactSize := bytes.Repeat([]byte("a"), int(maxConfigFileBytes))
	if err := os.WriteFile(path, exactSize, 0o600); err != nil {
		t.Fatalf("write exact-size config: %v", err)
	}

	raw, err := readLimitedFile(path, maxConfigFileBytes)
	if err != nil {
		t.Fatalf("readLimitedFile() error = %v", err)
	}
	if got := int64(len(raw)); got != maxConfigFileBytes {
		t.Fatalf("read bytes = %d, want %d", got, maxConfigFileBytes)
	}
}

func TestLoadPreservesExplicitWorktreeEnabledWhenMetadataParseFails(t *testing.T) {
	original := yamlUnmarshalConfigMetadataFn
	t.Cleanup(func() {
		yamlUnmarshalConfigMetadataFn = original
	})

	yamlUnmarshalConfigMetadataFn = func([]byte, *map[string]any) error {
		return errors.New("simulated metadata parse failure")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("worktree:\n  enabled: false\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Worktree.Enabled {
		t.Fatal("Worktree.Enabled should remain false when metadata parse fails")
	}
}

func TestLoadAppliesDefaultWorktreeEnabledWhenMetadataParseFailsAndEnabledMissing(t *testing.T) {
	original := yamlUnmarshalConfigMetadataFn
	t.Cleanup(func() {
		yamlUnmarshalConfigMetadataFn = original
	})

	yamlUnmarshalConfigMetadataFn = func([]byte, *map[string]any) error {
		return errors.New("simulated metadata parse failure")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := []byte("worktree:\n  setup_scripts:\n    - npm install\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Worktree.Enabled {
		t.Fatal("Worktree.Enabled should default to true when enabled is omitted")
	}
}

func TestProbeRawWorktreeEnabled(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		want    bool
		wantErr bool
	}{
		{
			name: "enabled true",
			raw:  []byte("worktree:\n  enabled: true\n"),
			want: true,
		},
		{
			name: "enabled false",
			raw:  []byte("worktree:\n  enabled: false\n"),
			want: true,
		},
		{
			name: "enabled missing",
			raw:  []byte("worktree:\n  setup_scripts:\n    - npm install\n"),
			want: false,
		},
		{
			name:    "invalid yaml",
			raw:     []byte("worktree: ["),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := probeRawWorktreeEnabled(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("probeRawWorktreeEnabled() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("probeRawWorktreeEnabled() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("probeRawWorktreeEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateConfigPathReturnsErrorWhenDefaultConfigDirResolutionFails(t *testing.T) {
	original := defaultConfigDirFn
	t.Cleanup(func() {
		defaultConfigDirFn = original
	})

	defaultConfigDirFn = func() (string, error) {
		return "", errors.New("simulated default dir error")
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := validateConfigPath(path); err == nil {
		t.Fatal("validateConfigPath() expected error when default config dir resolution fails")
	}
}

func TestAllowedShellList(t *testing.T) {
	shells := AllowedShellList()
	if len(shells) != len(allowedShells) {
		t.Fatalf("AllowedShellList() length = %d, want %d", len(shells), len(allowedShells))
	}
	for _, s := range shells {
		if _, ok := allowedShells[s]; !ok {
			t.Errorf("AllowedShellList() returned unexpected shell %q", s)
		}
	}
}

func TestAllowedShellListIsSorted(t *testing.T) {
	shells := AllowedShellList()
	for i := 1; i < len(shells); i++ {
		if shells[i-1] >= shells[i] {
			t.Errorf("AllowedShellList not sorted: %q >= %q at index %d", shells[i-1], shells[i], i)
		}
	}
}

func TestConfigStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[Config]().NumField(); got != 11 {
		t.Fatalf("Config field count = %d, want 11; update isZeroConfig tests for new fields", got)
	}
	if got := reflect.TypeFor[WorktreeConfig]().NumField(); got != 5 {
		t.Fatalf("WorktreeConfig field count = %d, want 5 (enabled, force_cleanup, setup_scripts, copy_files, copy_dirs)", got)
	}
	if got := reflect.TypeFor[ClaudeEnvConfig]().NumField(); got != 2 {
		t.Fatalf("ClaudeEnvConfig field count = %d, want 2 (default_enabled, vars); update Clone/sanitize for new fields", got)
	}
}

func TestCloneDeepCopyIndependence(t *testing.T) {
	src := DefaultConfig()
	src.Keys["custom-action"] = "a"
	src.Worktree.SetupScripts = []string{"script-a"}
	src.Worktree.CopyFiles = []string{".env"}
	src.Worktree.CopyDirs = []string{"vendor"}
	src.AgentModel = &AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
		Overrides: []AgentModelOverride{
			{Name: "security", Model: "claude-opus-4-6"},
		},
	}

	cloned := Clone(src)
	if &cloned.Keys == &src.Keys {
		t.Fatal("Clone() should deep-copy Keys map")
	}
	if &cloned.Worktree.SetupScripts == &src.Worktree.SetupScripts {
		t.Fatal("Clone() should deep-copy SetupScripts slice")
	}
	if &cloned.Worktree.CopyFiles == &src.Worktree.CopyFiles {
		t.Fatal("Clone() should deep-copy CopyFiles slice")
	}
	if &cloned.Worktree.CopyDirs == &src.Worktree.CopyDirs {
		t.Fatal("Clone() should deep-copy CopyDirs slice")
	}
	if cloned.AgentModel == src.AgentModel {
		t.Fatal("Clone() should deep-copy AgentModel pointer")
	}

	cloned.Keys["custom-action"] = "b"
	cloned.Worktree.SetupScripts[0] = "script-b"
	cloned.Worktree.CopyFiles[0] = ".env.local"
	cloned.Worktree.CopyDirs[0] = "node_modules"
	cloned.AgentModel.From = "changed-from"
	cloned.AgentModel.Overrides[0].Model = "changed-model"

	if src.Keys["custom-action"] != "a" {
		t.Fatalf("source Keys mutated: %q", src.Keys["custom-action"])
	}
	if src.Worktree.SetupScripts[0] != "script-a" {
		t.Fatalf("source SetupScripts mutated: %q", src.Worktree.SetupScripts[0])
	}
	if src.Worktree.CopyFiles[0] != ".env" {
		t.Fatalf("source CopyFiles mutated: %q", src.Worktree.CopyFiles[0])
	}
	if src.Worktree.CopyDirs[0] != "vendor" {
		t.Fatalf("source CopyDirs mutated: %q", src.Worktree.CopyDirs[0])
	}
	if src.AgentModel.From != "claude-opus-4-6" {
		t.Fatalf("source AgentModel.From mutated: %q", src.AgentModel.From)
	}
	if src.AgentModel.Overrides[0].Model != "claude-opus-4-6" {
		t.Fatalf("source AgentModel override mutated: %q", src.AgentModel.Overrides[0].Model)
	}
}

func TestClonePreservesNilCollections(t *testing.T) {
	src := Config{}
	cloned := Clone(src)

	if cloned.Keys != nil {
		t.Fatalf("Keys = %#v, want nil", cloned.Keys)
	}
	if cloned.Worktree.SetupScripts != nil {
		t.Fatalf("SetupScripts = %#v, want nil", cloned.Worktree.SetupScripts)
	}
	if cloned.Worktree.CopyFiles != nil {
		t.Fatalf("CopyFiles = %#v, want nil", cloned.Worktree.CopyFiles)
	}
	if cloned.Worktree.CopyDirs != nil {
		t.Fatalf("CopyDirs = %#v, want nil", cloned.Worktree.CopyDirs)
	}
	if cloned.AgentModel != nil {
		t.Fatalf("AgentModel = %#v, want nil", cloned.AgentModel)
	}
}

func TestClonePreservesNonNilEmptySlices(t *testing.T) {
	src := Config{}
	src.Worktree.SetupScripts = make([]string, 0)
	src.Worktree.CopyFiles = make([]string, 0)
	src.Worktree.CopyDirs = make([]string, 0)

	cloned := Clone(src)

	if cloned.Worktree.SetupScripts == nil {
		t.Fatal("SetupScripts = nil, want non-nil empty slice")
	}
	if cloned.Worktree.CopyFiles == nil {
		t.Fatal("CopyFiles = nil, want non-nil empty slice")
	}
	if cloned.Worktree.CopyDirs == nil {
		t.Fatal("CopyDirs = nil, want non-nil empty slice")
	}
	if len(cloned.Worktree.SetupScripts) != 0 {
		t.Fatalf("SetupScripts length = %d, want 0", len(cloned.Worktree.SetupScripts))
	}
	if len(cloned.Worktree.CopyFiles) != 0 {
		t.Fatalf("CopyFiles length = %d, want 0", len(cloned.Worktree.CopyFiles))
	}
	if len(cloned.Worktree.CopyDirs) != 0 {
		t.Fatalf("CopyDirs length = %d, want 0", len(cloned.Worktree.CopyDirs))
	}
}

func TestClonePreservesNilAgentModelOverrides(t *testing.T) {
	src := DefaultConfig()
	src.AgentModel = &AgentModel{
		From:      "claude-opus-4-6",
		To:        "claude-sonnet-4-5",
		Overrides: nil,
	}

	cloned := Clone(src)
	if cloned.AgentModel == nil {
		t.Fatal("Clone() AgentModel = nil, want non-nil")
	}
	if cloned.AgentModel.Overrides != nil {
		t.Fatalf("Clone() AgentModel.Overrides = %#v, want nil", cloned.AgentModel.Overrides)
	}

	cloned.AgentModel.Overrides = append(cloned.AgentModel.Overrides, AgentModelOverride{Name: "x", Model: "y"})
	if src.AgentModel.Overrides != nil {
		t.Fatalf("source AgentModel.Overrides mutated: %#v", src.AgentModel.Overrides)
	}
}

func TestClonePaneEnvDeepCopy(t *testing.T) {
	src := DefaultConfig()
	src.PaneEnv = map[string]string{
		"CLAUDE_CODE_EFFORT_LEVEL": "high",
		"CUSTOM_VAR":               "value",
	}

	cloned := Clone(src)
	if cloned.PaneEnv == nil {
		t.Fatal("Clone() PaneEnv = nil, want non-nil")
	}
	if len(cloned.PaneEnv) != 2 {
		t.Fatalf("Clone() PaneEnv len = %d, want 2", len(cloned.PaneEnv))
	}
	if cloned.PaneEnv["CLAUDE_CODE_EFFORT_LEVEL"] != "high" {
		t.Fatalf("Clone() PaneEnv[CLAUDE_CODE_EFFORT_LEVEL] = %q, want %q", cloned.PaneEnv["CLAUDE_CODE_EFFORT_LEVEL"], "high")
	}

	// Mutating cloned must not affect source.
	cloned.PaneEnv["NEW_KEY"] = "new"
	if _, exists := src.PaneEnv["NEW_KEY"]; exists {
		t.Fatal("source PaneEnv mutated after Clone modification")
	}
}

func TestClonePaneEnvNilPreserved(t *testing.T) {
	src := DefaultConfig()
	// PaneEnv is nil by default.
	cloned := Clone(src)
	if cloned.PaneEnv != nil {
		t.Fatalf("Clone() PaneEnv = %v, want nil", cloned.PaneEnv)
	}
}

func TestLoadPaneEnv(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantKeys []string
	}{
		{
			name: "pane_env with entries",
			yaml: `
pane_env:
  CLAUDE_CODE_EFFORT_LEVEL: "high"
  CUSTOM_VAR: "value"
`,
			wantKeys: []string{"CLAUDE_CODE_EFFORT_LEVEL", "CUSTOM_VAR"},
		},
		{
			name:     "pane_env omitted",
			yaml:     `shell: powershell.exe`,
			wantKeys: nil,
		},
		{
			name: "pane_env empty map",
			yaml: `
pane_env: {}
`,
			wantKeys: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if tt.wantKeys == nil {
				if len(cfg.PaneEnv) != 0 {
					t.Fatalf("PaneEnv = %v, want empty/nil", cfg.PaneEnv)
				}
				return
			}
			for _, key := range tt.wantKeys {
				if _, ok := cfg.PaneEnv[key]; !ok {
					t.Errorf("PaneEnv missing key %q", key)
				}
			}
		})
	}
}

func TestSanitizePaneEnv(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]string
		wantKeys   []string
		wantValues map[string]string // optional: verify specific values
	}{
		{
			name:       "normal entries preserved",
			input:      map[string]string{"FOO": "bar", "BAZ": "qux"},
			wantKeys:   []string{"FOO", "BAZ"},
			wantValues: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:       "empty key removed",
			input:      map[string]string{"": "value", "GOOD": "ok"},
			wantKeys:   []string{"GOOD"},
			wantValues: map[string]string{"GOOD": "ok"},
		},
		{
			name:       "whitespace-only key removed",
			input:      map[string]string{"  ": "value", "GOOD": "ok"},
			wantKeys:   []string{"GOOD"},
			wantValues: map[string]string{"GOOD": "ok"},
		},
		{
			name:     "null byte in key removed",
			input:    map[string]string{"BAD\x00KEY": "value", "GOOD": "ok"},
			wantKeys: []string{"GOOD"},
		},
		{
			name:       "values trimmed",
			input:      map[string]string{"KEY": "  trimmed  "},
			wantKeys:   []string{"KEY"},
			wantValues: map[string]string{"KEY": "trimmed"},
		},
		{
			name:       "null byte in value stripped",
			input:      map[string]string{"KEY": "abc\x00def"},
			wantKeys:   []string{"KEY"},
			wantValues: map[string]string{"KEY": "abcdef"},
		},
		{
			name:     "trim after duplicate key collapse (first-wins)",
			input:    map[string]string{" FOO ": "padded", "FOO": "exact"},
			wantKeys: []string{"FOO"},
			// NOTE: Go map iteration order is non-deterministic, so either value
			// could be the "first" one encountered. We only verify the key count
			// (exactly 1) to confirm the duplicate was detected and skipped.
		},
		{
			name:       "case-insensitive duplicate detection keeps first",
			input:      map[string]string{"MyVar": "first"},
			wantKeys:   []string{"MyVar"},
			wantValues: map[string]string{"MyVar": "first"},
		},
		{
			name:       "equals sign in key rejected",
			input:      map[string]string{"BAD=KEY": "value", "GOOD": "ok"},
			wantKeys:   []string{"GOOD"},
			wantValues: map[string]string{"GOOD": "ok"},
		},
		{
			name:     "all entries removed yields nil",
			input:    map[string]string{"": "a", "  ": "b"},
			wantKeys: nil,
		},
		{
			name:     "nil input",
			input:    nil,
			wantKeys: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{PaneEnv: tt.input}
			sanitizePaneEnv(cfg)
			if tt.wantKeys == nil {
				if len(cfg.PaneEnv) != 0 {
					t.Fatalf("PaneEnv = %v, want empty/nil", cfg.PaneEnv)
				}
				return
			}
			if len(cfg.PaneEnv) != len(tt.wantKeys) {
				t.Fatalf("PaneEnv len = %d, want %d; got %v", len(cfg.PaneEnv), len(tt.wantKeys), cfg.PaneEnv)
			}
			for _, key := range tt.wantKeys {
				if _, ok := cfg.PaneEnv[key]; !ok {
					t.Errorf("PaneEnv missing key %q", key)
				}
			}
			// Verify specific values when wantValues is provided.
			for k, wantV := range tt.wantValues {
				if gotV, ok := cfg.PaneEnv[k]; !ok {
					t.Errorf("PaneEnv missing key %q", k)
				} else if gotV != wantV {
					t.Errorf("PaneEnv[%q] = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestSanitizePaneEnvCaseInsensitiveDuplicate(t *testing.T) {
	// Dedicated test: two keys that differ only in case produce exactly one entry.
	// Because Go map iteration is non-deterministic, we only verify the count
	// and that the surviving key matches one of the original keys.
	cfg := &Config{PaneEnv: map[string]string{
		"MyVar": "first",
		"MYVAR": "second",
	}}
	sanitizePaneEnv(cfg)

	if len(cfg.PaneEnv) != 1 {
		t.Fatalf("PaneEnv len = %d, want 1; got %v", len(cfg.PaneEnv), cfg.PaneEnv)
	}
	// The surviving key should be either "MyVar" or "MYVAR" (first-wins, order undefined).
	for k := range cfg.PaneEnv {
		if !strings.EqualFold(k, "MYVAR") {
			t.Fatalf("unexpected surviving key %q", k)
		}
	}
}

func TestSanitizePaneEnvEqualsInKey(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		wantKeys []string
	}{
		{
			name:     "leading equals",
			input:    map[string]string{"=FOO": "val"},
			wantKeys: nil,
		},
		{
			name:     "trailing equals",
			input:    map[string]string{"FOO=": "val"},
			wantKeys: nil,
		},
		{
			name:     "embedded equals",
			input:    map[string]string{"FOO=BAR": "val"},
			wantKeys: nil,
		},
		{
			name:     "equals in value is allowed",
			input:    map[string]string{"FOO": "a=b"},
			wantKeys: []string{"FOO"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{PaneEnv: tt.input}
			sanitizePaneEnv(cfg)
			if tt.wantKeys == nil {
				if cfg.PaneEnv != nil {
					t.Fatalf("PaneEnv = %v, want nil", cfg.PaneEnv)
				}
				return
			}
			for _, key := range tt.wantKeys {
				if _, ok := cfg.PaneEnv[key]; !ok {
					t.Errorf("PaneEnv missing key %q", key)
				}
			}
		})
	}
}

func TestSanitizePaneEnvValueLengthWarning(t *testing.T) {
	// Verify that long values are preserved (not dropped) but a warning is logged.
	longValue := strings.Repeat("x", maxCustomEnvValueBytes+1)
	cfg := &Config{PaneEnv: map[string]string{"LONG_KEY": longValue}}
	sanitizePaneEnv(cfg)

	if cfg.PaneEnv == nil {
		t.Fatal("PaneEnv should not be nil; long values are warned but preserved")
	}
	if got := cfg.PaneEnv["LONG_KEY"]; got != longValue {
		t.Fatalf("PaneEnv[LONG_KEY] length = %d, want %d", len(got), len(longValue))
	}
}

func TestSanitizePaneEnvAllRemovedNormalizesToNil(t *testing.T) {
	// When all entries are invalid, PaneEnv should be normalized to nil.
	cfg := &Config{PaneEnv: map[string]string{
		"":           "empty-key",
		"\x00BAD":    "null-key",
		"HAS=EQUALS": "equals-key",
	}}
	sanitizePaneEnv(cfg)
	if cfg.PaneEnv != nil {
		t.Fatalf("PaneEnv = %v, want nil when all entries removed", cfg.PaneEnv)
	}
}

func TestEnsureFileCreatesConfigFile(t *testing.T) {
	path := newConfigPathForSaveTest(t, "config.yaml")

	if _, err := EnsureFile(path); err != nil {
		t.Fatalf("EnsureFile() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("EnsureFile() created a directory instead of file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config file permissions = %o, want owner-only", info.Mode().Perm())
	}
}

func TestEnsureFileUsesExistingConfigFile(t *testing.T) {
	path := newConfigPathForSaveTest(t, "config.yaml")
	initial := []byte("shell: cmd.exe\nprefix: Ctrl+a\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := EnsureFile(path)
	if err != nil {
		t.Fatalf("EnsureFile() error = %v", err)
	}
	if cfg.Shell != "cmd.exe" {
		t.Fatalf("cfg.Shell = %q, want cmd.exe", cfg.Shell)
	}
	if cfg.Prefix != "Ctrl+a" {
		t.Fatalf("cfg.Prefix = %q, want Ctrl+a", cfg.Prefix)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(raw), "cmd.exe") {
		t.Fatalf("existing config was unexpectedly replaced: %q", string(raw))
	}
}

func TestLoadReturnsDefaultsOnParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("worktree: ["), 0o600); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected parse error")
	}

	want := DefaultConfig()
	if cfg.Shell != want.Shell {
		t.Fatalf("cfg.Shell = %q, want default %q", cfg.Shell, want.Shell)
	}
	if cfg.Prefix != want.Prefix {
		t.Fatalf("cfg.Prefix = %q, want default %q", cfg.Prefix, want.Prefix)
	}
	if cfg.Worktree.Enabled != want.Worktree.Enabled {
		t.Fatalf("cfg.Worktree.Enabled = %v, want default %v", cfg.Worktree.Enabled, want.Worktree.Enabled)
	}
}

func TestEnsureFileReturnsLoadedConfigWhenInitialSaveFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outside-default-config-dir.yaml")
	cfg, err := EnsureFile(path)
	if err == nil {
		t.Fatal("EnsureFile() expected save error for path outside default config dir")
	}
	want := DefaultConfig()
	if cfg.Shell != want.Shell {
		t.Fatalf("cfg.Shell = %q, want default %q", cfg.Shell, want.Shell)
	}
}

func TestSaveConcurrentWrites(t *testing.T) {
	path := newConfigPathForSaveTest(t, "concurrent-config.yaml")

	const writers = 6
	const iterations = 30

	var wg sync.WaitGroup
	errCh := make(chan error, writers*iterations)

	for i := range writers {
		writerID := i
		wg.Go(func() {
			for j := range iterations {
				cfg := DefaultConfig()
				if (writerID+j)%2 == 0 {
					cfg.Shell = "cmd.exe"
				} else {
					cfg.Shell = "pwsh.exe"
				}
				if _, err := Save(path, cfg); err != nil {
					errCh <- err
					return
				}
			}
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Save() concurrent write error = %v", err)
		}
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() after concurrent writes error = %v", err)
	}
	if loaded.Shell != "cmd.exe" && loaded.Shell != "pwsh.exe" {
		t.Fatalf("final shell = %q, want cmd.exe or pwsh.exe", loaded.Shell)
	}
}

func TestCloneClaudeEnv(t *testing.T) {
	t.Run("nil ClaudeEnv stays nil", func(t *testing.T) {
		src := DefaultConfig()
		dst := Clone(src)
		if dst.ClaudeEnv != nil {
			t.Fatal("ClaudeEnv should be nil")
		}
	})
	t.Run("deep copy independence", func(t *testing.T) {
		src := DefaultConfig()
		src.ClaudeEnv = &ClaudeEnvConfig{
			DefaultEnabled: true,
			Vars:           map[string]string{"KEY": "val"},
		}
		dst := Clone(src)
		if dst.ClaudeEnv == src.ClaudeEnv {
			t.Fatal("ClaudeEnv should be different pointer")
		}
		dst.ClaudeEnv.Vars["KEY"] = "modified"
		if src.ClaudeEnv.Vars["KEY"] != "val" {
			t.Fatal("modifying clone mutated source")
		}
		dst.ClaudeEnv.DefaultEnabled = false
		if !src.ClaudeEnv.DefaultEnabled {
			t.Fatal("modifying clone's DefaultEnabled mutated source")
		}
	})
	t.Run("nil vars stays nil", func(t *testing.T) {
		src := DefaultConfig()
		src.ClaudeEnv = &ClaudeEnvConfig{DefaultEnabled: true}
		dst := Clone(src)
		if dst.ClaudeEnv.Vars != nil {
			t.Fatal("Vars should be nil")
		}
		if !dst.ClaudeEnv.DefaultEnabled {
			t.Fatal("DefaultEnabled should be true")
		}
	})
}

func TestSanitizeClaudeEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    *ClaudeEnvConfig
		wantVars map[string]string
	}{
		{
			name:  "nil config",
			input: nil,
		},
		{
			name:     "empty vars",
			input:    &ClaudeEnvConfig{Vars: map[string]string{}},
			wantVars: nil, // normalized to nil
		},
		{
			name:     "blocked key warned but preserved",
			input:    &ClaudeEnvConfig{Vars: map[string]string{"PATH": "/usr/bin", "MY_KEY": "val"}},
			wantVars: map[string]string{"PATH": "/usr/bin", "MY_KEY": "val"},
		},
		{
			name:     "null byte in key removed",
			input:    &ClaudeEnvConfig{Vars: map[string]string{"KEY\x00BAD": "val"}},
			wantVars: nil,
		},
		{
			name:     "null byte in value stripped",
			input:    &ClaudeEnvConfig{Vars: map[string]string{"KEY": "val\x00ue"}},
			wantVars: map[string]string{"KEY": "value"},
		},
		{
			name:     "equals in key removed",
			input:    &ClaudeEnvConfig{Vars: map[string]string{"K=V": "val"}},
			wantVars: nil,
		},
		{
			name:     "case insensitive dedup keeps first",
			input:    &ClaudeEnvConfig{Vars: map[string]string{"key": "lower", "KEY": "upper"}},
			wantVars: map[string]string{"KEY": "upper"}, // sorted: KEY < key, so KEY wins
		},
		{
			name:     "DefaultEnabled preserved when vars emptied",
			input:    &ClaudeEnvConfig{DefaultEnabled: true, Vars: map[string]string{"K=V": "/bad"}},
			wantVars: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ClaudeEnv: tt.input}
			sanitizeClaudeEnv(cfg)
			if tt.input == nil {
				if cfg.ClaudeEnv != nil {
					t.Fatal("ClaudeEnv should remain nil")
				}
				return
			}
			if !reflect.DeepEqual(cfg.ClaudeEnv.Vars, tt.wantVars) {
				t.Errorf("Vars = %v, want %v", cfg.ClaudeEnv.Vars, tt.wantVars)
			}
			// Verify DefaultEnabled is preserved
			if tt.name == "DefaultEnabled preserved when vars emptied" {
				if !cfg.ClaudeEnv.DefaultEnabled {
					t.Error("DefaultEnabled should be preserved")
				}
			}
		})
	}
}

// TestSanitizeEnvMap tests environment variable sanitization via the production
// sanitizeEnvMap function. This test lives in config_test.go (package config)
// because sanitizeEnvMap is intentionally unexported.
func TestSanitizeEnvMap(t *testing.T) {
	tests := []struct {
		name    string
		entries map[string]string
		want    map[string]string
		wantNil bool
	}{
		{
			name:    "nil input returns nil",
			entries: nil,
			want:    nil,
			wantNil: true,
		},
		{
			name:    "empty map returns nil",
			entries: map[string]string{},
			want:    nil,
			wantNil: true,
		},
		{
			name:    "single valid entry",
			entries: map[string]string{"KEY": "value"},
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "empty key dropped",
			entries: map[string]string{"": "value"},
			want:    nil,
			wantNil: true,
		},
		{
			name: "whitespace-only key dropped",
			entries: map[string]string{
				"   ": "value",
				"KEY": "value",
			},
			want: map[string]string{"KEY": "value"},
		},
		{
			name: "null byte in key dropped",
			entries: map[string]string{
				"KEY\x00EVIL": "value",
				"VALID":       "value",
			},
			want: map[string]string{"VALID": "value"},
		},
		{
			name: "equals sign in key dropped",
			entries: map[string]string{
				"KEY=INVALID": "value",
				"VALID":       "value",
			},
			want: map[string]string{"VALID": "value"},
		},
		{
			name: "null bytes in value stripped",
			entries: map[string]string{
				"KEY": "val\x00ue",
			},
			want: map[string]string{"KEY": "value"},
		},
		{
			// Production sorts keys alphabetically before iterating, so
			// "KEY" < "Key" < "key". The first alphabetical key wins.
			name: "case-insensitive duplicate detection keeps first alphabetically",
			entries: map[string]string{
				"KEY": "first",
				"key": "second",
				"Key": "third",
			},
			want: map[string]string{"KEY": "first"},
		},
		{
			name: "whitespace trimmed from value",
			entries: map[string]string{
				"KEY": "  value  ",
			},
			want: map[string]string{"KEY": "value"},
		},
		{
			name:    "key with empty value keeps entry",
			entries: map[string]string{"KEY": ""},
			want:    map[string]string{"KEY": ""},
		},
		{
			name: "mixed valid and invalid",
			entries: map[string]string{
				"VALID1": "value1",
				"":       "dropped",
				"VALID2": "value2",
				"BAD=":   "dropped",
			},
			want: map[string]string{
				"VALID1": "value1",
				"VALID2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeEnvMap(tt.entries, "test")
			if tt.wantNil && got != nil {
				t.Errorf("sanitizeEnvMap() = %v, want nil", got)
				return
			}
			if !tt.wantNil && got == nil {
				t.Errorf("sanitizeEnvMap() = nil, want non-nil")
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sanitizeEnvMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadSaveClaudeEnv(t *testing.T) {
	t.Run("round trip with vars", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := DefaultConfig()
		cfg.ClaudeEnv = &ClaudeEnvConfig{
			DefaultEnabled: true,
			Vars:           map[string]string{"ANTHROPIC_API_KEY": "sk-test", "CLAUDE_CODE_EFFORT_LEVEL": "high"},
		}
		cfg.PaneEnvDefaultEnabled = true
		if _, err := Save(path, cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.ClaudeEnv == nil {
			t.Fatal("ClaudeEnv is nil after round-trip")
		}
		if !loaded.ClaudeEnv.DefaultEnabled {
			t.Error("DefaultEnabled should be true")
		}
		if len(loaded.ClaudeEnv.Vars) != 2 {
			t.Errorf("Vars count = %d, want 2", len(loaded.ClaudeEnv.Vars))
		}
		if loaded.ClaudeEnv.Vars["ANTHROPIC_API_KEY"] != "sk-test" {
			t.Errorf("ANTHROPIC_API_KEY = %q", loaded.ClaudeEnv.Vars["ANTHROPIC_API_KEY"])
		}
		if !loaded.PaneEnvDefaultEnabled {
			t.Error("PaneEnvDefaultEnabled should be true")
		}
	})

	t.Run("default_enabled false serialized", func(t *testing.T) {
		path := newConfigPathForSaveTest(t, "config.yaml")
		cfg := DefaultConfig()
		cfg.ClaudeEnv = &ClaudeEnvConfig{
			DefaultEnabled: false,
			Vars:           map[string]string{"MY_KEY": "val"},
		}
		if _, err := Save(path, cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.ClaudeEnv == nil {
			t.Fatal("ClaudeEnv is nil")
		}
		if loaded.ClaudeEnv.DefaultEnabled {
			t.Error("DefaultEnabled should be false")
		}
	})

	t.Run("omitted claude_env loads as nil", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		raw := []byte("shell: powershell.exe\n")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.ClaudeEnv != nil {
			t.Errorf("ClaudeEnv should be nil when omitted, got %+v", loaded.ClaudeEnv)
		}
		if loaded.PaneEnvDefaultEnabled {
			t.Error("PaneEnvDefaultEnabled should be false when omitted")
		}
	})
}

// TestValidateWebSocketPort verifies port range validation (0-65535).
// Port 0 means "auto-assign"; values outside the range fall back to 0.
func TestValidateWebSocketPort(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantPort int
	}{
		{name: "valid port 8080", port: 8080, wantPort: 8080},
		{name: "port 0 auto-assign", port: 0, wantPort: 0},
		{name: "port 1 minimum usable", port: 1, wantPort: 1},
		{name: "port 65535 max valid", port: 65535, wantPort: 65535},
		{name: "port 65536 exceeds max falls back to 0", port: 65536, wantPort: 0},
		{name: "negative port falls back to 0", port: -1, wantPort: 0},
		{name: "large negative falls back to 0", port: -9999, wantPort: 0},
		{name: "very large port falls back to 0", port: 100000, wantPort: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{WebSocketPort: tt.port}
			validateWebSocketPort(cfg)
			if cfg.WebSocketPort != tt.wantPort {
				t.Errorf("validateWebSocketPort() port = %d, want %d", cfg.WebSocketPort, tt.wantPort)
			}
		})
	}
}

// TestWebSocketPortSaveRoundTrip verifies that saving a config with a non-zero
// WebSocketPort and loading it back preserves the value.
// S-23: Ensures the websocket_port field survives the Save -> Load cycle.
func TestWebSocketPortSaveRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantPort int
	}{
		{name: "port 0 auto-assign", port: 0, wantPort: 0},
		{name: "port 8080", port: 8080, wantPort: 8080},
		{name: "port 65535 max valid", port: 65535, wantPort: 65535},
		{name: "port 1 minimum usable", port: 1, wantPort: 1},
		{name: "port 443 common HTTPS", port: 443, wantPort: 443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := newConfigPathForSaveTest(t, "config.yaml")
			cfg := DefaultConfig()
			cfg.WebSocketPort = tt.port

			if _, err := Save(path, cfg); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			loaded, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if loaded.WebSocketPort != tt.wantPort {
				t.Errorf("WebSocketPort = %d, want %d after round-trip", loaded.WebSocketPort, tt.wantPort)
			}
		})
	}
}

// TestLoadWebSocketPortValidation verifies that Load applies port range
// validation end-to-end: invalid ports in config files fall back to 0.
func TestLoadWebSocketPortValidation(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantPort int
	}{
		{name: "valid port preserved", yaml: "websocket_port: 8080\n", wantPort: 8080},
		{name: "port 0 preserved", yaml: "websocket_port: 0\n", wantPort: 0},
		{name: "max port preserved", yaml: "websocket_port: 65535\n", wantPort: 65535},
		{name: "port exceeding max falls back", yaml: "websocket_port: 65536\n", wantPort: 0},
		{name: "negative port falls back", yaml: "websocket_port: -1\n", wantPort: 0},
		{name: "omitted port defaults to 0", yaml: "shell: powershell.exe\n", wantPort: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.WebSocketPort != tt.wantPort {
				t.Errorf("WebSocketPort = %d, want %d", cfg.WebSocketPort, tt.wantPort)
			}
		})
	}
}
