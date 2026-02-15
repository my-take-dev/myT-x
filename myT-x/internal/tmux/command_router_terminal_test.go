package tmux

import (
	"os"
	"sort"
	"strings"
	"testing"

	"myT-x/internal/config"
	"myT-x/internal/ipc"
)

func TestMergeEnvironmentBlocksSystemKeys(t *testing.T) {
	basePath := os.Getenv("PATH")
	merged := mergeEnvironment(map[string]string{
		"PATH":     `C:\attacker`,
		"SAFE_KEY": "safe",
	})
	values := envSliceToMap(merged)

	if values["PATH"] != basePath {
		t.Fatalf("PATH should not be overridden, got %q", values["PATH"])
	}
	if values["SAFE_KEY"] != "safe" {
		t.Fatalf("SAFE_KEY = %q, want safe", values["SAFE_KEY"])
	}
}

func TestMergeEnvironmentSanitizesNullAndLength(t *testing.T) {
	longValue := strings.Repeat("x", maxCustomEnvValueBytes+128)
	merged := mergeEnvironment(map[string]string{
		"NULL_VALUE": "abc\x00def",
		"LONG_VALUE": longValue,
	})
	values := envSliceToMap(merged)

	if strings.ContainsRune(values["NULL_VALUE"], '\x00') {
		t.Fatalf("NULL_VALUE still contains null byte")
	}
	if got := len(values["LONG_VALUE"]); got != maxCustomEnvValueBytes {
		t.Fatalf("LONG_VALUE length = %d, want %d", got, maxCustomEnvValueBytes)
	}
}

func TestMergeEnvironmentBlocksCaseInsensitiveKeys(t *testing.T) {
	basePath := os.Getenv("PATH")
	merged := mergeEnvironment(map[string]string{
		"path":    `C:\attacker`,
		"Path":    `C:\attacker2`,
		"comspec": `C:\evil.exe`,
	})
	values := envSliceToMap(merged)

	if values["PATH"] != basePath {
		t.Fatalf("PATH should not be overridden by lowercase key, got %q", values["PATH"])
	}
}

func TestMergeEnvironmentRejectsKeyWithEquals(t *testing.T) {
	merged := mergeEnvironment(map[string]string{
		"FOO=BAR": "injected",
		"GOOD":    "ok",
	})
	values := envSliceToMap(merged)

	if _, found := values["FOO=BAR"]; found {
		t.Fatalf("key containing '=' should be rejected")
	}
	if values["GOOD"] != "ok" {
		t.Fatalf("GOOD = %q, want ok", values["GOOD"])
	}
}

func TestMergeEnvironmentRejectsEmptyAndWhitespaceKey(t *testing.T) {
	merged := mergeEnvironment(map[string]string{
		"":    "empty",
		"   ": "whitespace",
		"OK":  "valid",
	})
	values := envSliceToMap(merged)

	if _, found := values[""]; found {
		t.Fatalf("empty key should be rejected")
	}
	if values["OK"] != "valid" {
		t.Fatalf("OK = %q, want valid", values["OK"])
	}
}

func TestMergeEnvironmentRejectsKeyWithNullByte(t *testing.T) {
	merged := mergeEnvironment(map[string]string{
		"FOO\x00BAR": "injected",
		"GOOD":       "ok",
	})
	values := envSliceToMap(merged)

	if _, found := values["FOO\x00BAR"]; found {
		t.Fatalf("key containing null byte should be rejected")
	}
	if _, found := values["FOO"]; found {
		t.Fatalf("truncated null-byte key should not appear")
	}
	if values["GOOD"] != "ok" {
		t.Fatalf("GOOD = %q, want ok", values["GOOD"])
	}
}

func TestMergeEnvironmentBlocksNewlyAddedKeys(t *testing.T) {
	attackerValues := map[string]string{
		"PATHEXT":      ".evil",
		"LOCALAPPDATA": `C:\attacker`,
		"PSMODULEPATH": `C:\attacker\modules`,
		"TEMP":         `C:\attacker\temp`,
	}
	merged := mergeEnvironment(attackerValues)
	values := envSliceToMap(merged)

	for key, attackerValue := range attackerValues {
		if got, found := values[key]; found && got == attackerValue {
			t.Fatalf("%s was overridden to attacker value %q", key, got)
		}
	}
}

func TestShimAvailableGetterSetter(t *testing.T) {
	tests := []struct {
		name    string
		initial bool
		setTo   bool
		wantGet bool
	}{
		{"default false", false, false, false},
		{"initial true", true, true, true},
		{"set true at runtime", false, true, true},
		{"set false at runtime", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(nil, nil, RouterOptions{
				ShimAvailable: tt.initial,
			})
			router.SetShimAvailable(tt.setTo)
			if got := router.ShimAvailable(); got != tt.wantGet {
				t.Errorf("ShimAvailable() = %v, want %v", got, tt.wantGet)
			}
		})
	}
}

func TestAddTmuxEnvironmentAlwaysSetsTMUX(t *testing.T) {
	tests := []struct {
		name          string
		shimAvailable bool
	}{
		{"shim available", true},
		{"shim not available", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{}
			addTmuxEnvironment(env, `\\.\pipe\test`, 12345, 0, 1, tt.shimAvailable)

			wantTmuxVal := `\\.\pipe\test,12345,0`

			// GO_TMUX / GO_TMUX_PANE は常に設定される
			if got := env["GO_TMUX"]; got != wantTmuxVal {
				t.Errorf("GO_TMUX = %q, want %q", got, wantTmuxVal)
			}
			if got := env["GO_TMUX_PANE"]; got != "%1" {
				t.Errorf("GO_TMUX_PANE = %q, want %%1", got)
			}

			// TMUX / TMUX_PANE は shimAvailable に関係なく常に設定される
			// (本物の tmux と同様の動作)
			if got := env["TMUX"]; got != wantTmuxVal {
				t.Errorf("TMUX = %q, want %q", got, wantTmuxVal)
			}
			if got := env["TMUX_PANE"]; got != "%1" {
				t.Errorf("TMUX_PANE = %q, want %%1", got)
			}
		})
	}
}

func TestSplitWindowDetachFlag(t *testing.T) {
	sessions := NewSessionManager()
	router := NewCommandRouter(sessions, nil, RouterOptions{
		ShimAvailable: true,
	})

	// Create a session first
	createResp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{"-s": "test-detach", "-d": true},
	})
	if createResp.ExitCode != 0 {
		t.Fatalf("new-session failed: %s", createResp.Stderr)
	}

	// Split without -d: new pane should be active
	splitResp := router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{"-t": "test-detach:0", "-h": true},
	})
	if splitResp.ExitCode != 0 {
		t.Fatalf("split-window failed: %s", splitResp.Stderr)
	}
	newPaneID := strings.TrimSpace(splitResp.Stdout)

	// The new pane should be active (default behavior)
	pane, err := sessions.ResolveTarget(newPaneID, -1)
	if err != nil {
		t.Fatalf("resolve new pane: %v", err)
	}
	if !pane.Active {
		t.Error("new pane should be active without -d flag")
	}

	// Split with -d: original pane should remain active
	splitResp2 := router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{"-t": newPaneID, "-h": true, "-d": true},
	})
	if splitResp2.ExitCode != 0 {
		t.Fatalf("split-window -d failed: %s", splitResp2.Stderr)
	}

	// The target pane should still be active (focus preserved by -d)
	paneAfter, err := sessions.ResolveTarget(newPaneID, -1)
	if err != nil {
		t.Fatalf("resolve target pane after -d: %v", err)
	}
	if !paneAfter.Active {
		t.Error("target pane should remain active with -d flag")
	}
}

func TestSplitWindowPrintFormat(t *testing.T) {
	sessions := NewSessionManager()
	router := NewCommandRouter(sessions, nil, RouterOptions{
		ShimAvailable: true,
	})

	// Create a session
	createResp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{"-s": "test-pf", "-d": true},
	})
	if createResp.ExitCode != 0 {
		t.Fatalf("new-session failed: %s", createResp.Stderr)
	}

	tests := []struct {
		name       string
		flags      map[string]any
		wantFormat string // substring to check in output
	}{
		{
			name:       "no -P: returns pane ID only",
			flags:      map[string]any{"-t": "test-pf:0", "-h": true},
			wantFormat: "%",
		},
		{
			name:       "-P without -F: returns pane ID (default format)",
			flags:      map[string]any{"-t": "test-pf:0", "-h": true, "-P": true},
			wantFormat: "%",
		},
		{
			name:       "-P with -F pane_id: returns pane ID",
			flags:      map[string]any{"-t": "test-pf:0", "-h": true, "-P": true, "-F": "#{pane_id}"},
			wantFormat: "%",
		},
		{
			name:       "-P with -F session_name: returns session name",
			flags:      map[string]any{"-t": "test-pf:0", "-h": true, "-P": true, "-F": "#{session_name}"},
			wantFormat: "test-pf",
		},
		{
			name:       "-P with -F compound: returns compound format",
			flags:      map[string]any{"-t": "test-pf:0", "-h": true, "-P": true, "-F": "#{session_name}:#{pane_id}"},
			wantFormat: "test-pf:%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := router.Execute(ipc.TmuxRequest{
				Command: "split-window",
				Flags:   tt.flags,
			})
			if resp.ExitCode != 0 {
				t.Fatalf("split-window failed: %s", resp.Stderr)
			}
			output := strings.TrimSpace(resp.Stdout)
			if !strings.Contains(output, tt.wantFormat) {
				t.Errorf("output = %q, want containing %q", output, tt.wantFormat)
			}
		})
	}
}

func TestNewSessionPrintFormat(t *testing.T) {
	sessions := NewSessionManager()
	router := NewCommandRouter(sessions, nil, RouterOptions{
		ShimAvailable: true,
	})

	tests := []struct {
		name       string
		flags      map[string]any
		wantSubstr string
	}{
		{
			name:       "no -P: returns session name",
			flags:      map[string]any{"-s": "ns-plain", "-d": true},
			wantSubstr: "ns-plain",
		},
		{
			name:       "-P with -F pane_id",
			flags:      map[string]any{"-s": "ns-pf", "-d": true, "-P": true, "-F": "#{pane_id}"},
			wantSubstr: "%",
		},
		{
			name:       "-P with -F session_name:pane_id",
			flags:      map[string]any{"-s": "ns-compound", "-d": true, "-P": true, "-F": "#{session_name}:#{pane_id}"},
			wantSubstr: "ns-compound:%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := router.Execute(ipc.TmuxRequest{
				Command: "new-session",
				Flags:   tt.flags,
			})
			if resp.ExitCode != 0 {
				t.Fatalf("new-session failed: %s", resp.Stderr)
			}
			output := strings.TrimSpace(resp.Stdout)
			if !strings.Contains(output, tt.wantSubstr) {
				t.Errorf("output = %q, want containing %q", output, tt.wantSubstr)
			}
		})
	}
}

func TestMergePaneEnvDefaults(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		paneEnv map[string]string
		wantEnv map[string]string
	}{
		{
			name:    "merges new keys",
			env:     map[string]string{"EXISTING": "val"},
			paneEnv: map[string]string{"NEW_KEY": "new_val"},
			wantEnv: map[string]string{"EXISTING": "val", "NEW_KEY": "new_val"},
		},
		{
			name:    "does not overwrite existing keys",
			env:     map[string]string{"KEY": "original"},
			paneEnv: map[string]string{"KEY": "pane_default"},
			wantEnv: map[string]string{"KEY": "original"},
		},
		{
			name:    "nil paneEnv is no-op",
			env:     map[string]string{"EXISTING": "val"},
			paneEnv: nil,
			wantEnv: map[string]string{"EXISTING": "val"},
		},
		{
			name:    "empty paneEnv is no-op",
			env:     map[string]string{"EXISTING": "val"},
			paneEnv: map[string]string{},
			wantEnv: map[string]string{"EXISTING": "val"},
		},
		{
			name:    "empty env receives paneEnv",
			env:     map[string]string{},
			paneEnv: map[string]string{"FOO": "bar", "BAZ": "qux"},
			wantEnv: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:    "mixed: some keys exist, some new",
			env:     map[string]string{"KEEP": "original", "ALSO_KEEP": "original2"},
			paneEnv: map[string]string{"KEEP": "should_not_override", "NEW": "added"},
			wantEnv: map[string]string{"KEEP": "original", "ALSO_KEEP": "original2", "NEW": "added"},
		},
		{
			name:    "case-sensitive: different case keys coexist",
			env:     map[string]string{"path": "/usr/bin"},
			paneEnv: map[string]string{"PATH": "/pane/bin", "path": "should_not_override"},
			wantEnv: map[string]string{"path": "/usr/bin", "PATH": "/pane/bin"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergePaneEnvDefaults(tt.env, tt.paneEnv)
			if len(tt.env) != len(tt.wantEnv) {
				t.Fatalf("env len = %d, want %d; got %v", len(tt.env), len(tt.wantEnv), tt.env)
			}
			for k, want := range tt.wantEnv {
				if got, ok := tt.env[k]; !ok {
					t.Errorf("env missing key %q", k)
				} else if got != want {
					t.Errorf("env[%q] = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestMergePaneEnvDefaultsNilEnv(t *testing.T) {
	// nil env must not panic — early return guards against nil map write.
	mergePaneEnvDefaults(nil, map[string]string{"KEY": "val"})
}

func TestMergeEnvironmentBlocksPaneEnvBlockedKeys(t *testing.T) {
	// Simulate the full flow: paneEnv → mergePaneEnvDefaults → mergeEnvironment.
	// Blocked keys set via paneEnv must be rejected by mergeEnvironment.
	env := map[string]string{"SAFE": "ok"}
	paneEnv := map[string]string{
		"PATH":       "/evil",
		"COMSPEC":    "C:\\evil.exe",
		"SYSTEMROOT": "C:\\evil",
		"CUSTOM_VAR": "allowed",
	}
	mergePaneEnvDefaults(env, paneEnv)

	merged := mergeEnvironment(env)
	values := envSliceToMap(merged)

	// Blocked keys must retain their original system values, not the paneEnv values.
	if values["PATH"] == "/evil" {
		t.Fatal("PATH should not be overridden by paneEnv value")
	}
	if values["COMSPEC"] == "C:\\evil.exe" {
		t.Fatal("COMSPEC should not be overridden by paneEnv value")
	}
	if values["SYSTEMROOT"] == "C:\\evil" {
		t.Fatal("SYSTEMROOT should not be overridden by paneEnv value")
	}

	// Non-blocked keys should pass through.
	if values["SAFE"] != "ok" {
		t.Fatalf("SAFE = %q, want %q", values["SAFE"], "ok")
	}
	if values["CUSTOM_VAR"] != "allowed" {
		t.Fatalf("CUSTOM_VAR = %q, want %q", values["CUSTOM_VAR"], "allowed")
	}
}

func envSliceToMap(items []string) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

// TestBlockedKeyListsMatch is a guard test that ensures config.warnOnlyBlockedKeys
// and tmux.blockedEnvironmentKeys contain exactly the same key set.
// INVARIANT: frontend settingsValidation.ts BLOCKED_ENV_KEYS must also match
// (verified manually or via integration test, as TS is outside Go test scope).
func TestBlockedKeyListsMatch(t *testing.T) {
	configKeys := config.BlockedKeyNames()
	tmuxKeys := blockedEnvironmentKeys

	// Extract sorted key slices for readable diff on failure.
	configSorted := make([]string, 0, len(configKeys))
	for k := range configKeys {
		configSorted = append(configSorted, k)
	}
	sort.Strings(configSorted)

	tmuxSorted := make([]string, 0, len(tmuxKeys))
	for k := range tmuxKeys {
		tmuxSorted = append(tmuxSorted, k)
	}
	sort.Strings(tmuxSorted)

	if len(configSorted) != len(tmuxSorted) {
		t.Fatalf("blocked key count mismatch: config=%d (%v), tmux=%d (%v)",
			len(configSorted), configSorted, len(tmuxSorted), tmuxSorted)
	}
	for i := range configSorted {
		if configSorted[i] != tmuxSorted[i] {
			t.Errorf("blocked key mismatch at index %d: config=%q, tmux=%q\nconfig keys: %v\ntmux keys:   %v",
				i, configSorted[i], tmuxSorted[i], configSorted, tmuxSorted)
		}
	}
}
