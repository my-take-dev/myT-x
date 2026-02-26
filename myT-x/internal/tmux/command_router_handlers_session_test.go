package tmux

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestHandleNewSessionRollbackFailureHidesRollbackError(t *testing.T) {
	emitter := &captureEmitter{}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)

	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
	attachErr := errors.New("attach failed")
	router.attachTerminalFn = func(pane *TmuxPane, _ string, _ map[string]string, _ *TmuxPane) error {
		if pane == nil || pane.Window == nil || pane.Window.Session == nil {
			t.Fatal("pane/session context missing in attach hook")
		}
		if _, rmErr := sessions.RemoveSession(pane.Window.Session.Name); rmErr != nil {
			t.Fatalf("RemoveSession() error = %v", rmErr)
		}
		return attachErr
	}

	resp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-s": "rollback-demo",
			"-n": "main",
		},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	if !strings.Contains(resp.Stderr, attachErr.Error()) {
		t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, attachErr.Error())
	}
	if strings.Contains(resp.Stderr, "rollback failed") {
		t.Fatalf("Stderr should not leak rollback internals: %q", resp.Stderr)
	}
	if strings.Contains(resp.Stderr, "session not found") {
		t.Fatalf("Stderr should not leak rollback RemoveSession error: %q", resp.Stderr)
	}
}

func TestHandleAttachSession(t *testing.T) {
	tests := []struct {
		name          string
		sessionName   string
		target        string
		wantExitCode  int
		wantEventName string
		wantStderr    string
	}{
		{
			name:          "existing session returns success and emits activate",
			sessionName:   "multiagent",
			target:        "multiagent",
			wantExitCode:  0,
			wantEventName: "app:activate-window",
		},
		{
			name:          "session:window target resolves to existing session",
			sessionName:   "multiagent",
			target:        "multiagent:0",
			wantExitCode:  0,
			wantEventName: "app:activate-window",
		},
		{
			name:          "session:window.pane target resolves to existing session",
			sessionName:   "multiagent",
			target:        "multiagent:0.0",
			wantExitCode:  0,
			wantEventName: "app:activate-window",
		},
		{
			name:         "non-existent session returns error",
			sessionName:  "multiagent",
			target:       "nonexistent",
			wantExitCode: 1,
			wantStderr:   "session not found: nonexistent",
		},
		{
			name:         "whitespace-only target returns error",
			sessionName:  "multiagent",
			target:       "   ",
			wantExitCode: 1,
			wantStderr:   "missing required flag: -t",
		},
		{
			name:         "empty target returns error",
			sessionName:  "multiagent",
			target:       "",
			wantExitCode: 1,
			wantStderr:   "missing required flag: -t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession(tt.sessionName, "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "attach-session",
				Flags:   map[string]any{"-t": tt.target},
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantExitCode == 0 && resp.Stdout != "" {
				t.Fatalf("Stdout = %q, want empty stdout on success", resp.Stdout)
			}
			if tt.wantStderr != "" && !strings.Contains(resp.Stderr, tt.wantStderr) {
				t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
			}

			events := emitter.Events()
			if tt.wantEventName == "" {
				if len(events) != 0 {
					t.Fatalf("unexpected events emitted: %v", events)
				}
				return
			}

			matchCount := 0
			for _, ev := range events {
				if ev.name == tt.wantEventName {
					matchCount++
					if ev.payload != nil {
						t.Fatalf("event payload = %#v, want nil", ev.payload)
					}
				}
			}
			if matchCount == 0 {
				t.Fatalf("expected event %q not found in %v", tt.wantEventName, events)
			}
			if matchCount != 1 {
				t.Fatalf("event %q count = %d, want 1", tt.wantEventName, matchCount)
			}
		})
	}
}

func TestHandleKillSession(t *testing.T) {
	tests := []struct {
		name         string
		target       string
		wantExitCode int
		wantStderr   string
		wantEvent    string
		wantName     string
	}{
		{
			name:         "kills session by exact name",
			target:       "demo",
			wantExitCode: 0,
			wantEvent:    "tmux:session-destroyed",
			wantName:     "demo",
		},
		{
			name:         "kills session by session:window target",
			target:       "demo:0",
			wantExitCode: 0,
			wantEvent:    "tmux:session-destroyed",
			wantName:     "demo",
		},
		{
			name:         "missing target returns error",
			target:       "   ",
			wantExitCode: 1,
			wantStderr:   "missing required flag: -t",
		},
		{
			name:         "unknown session returns error",
			target:       "missing:0",
			wantExitCode: 1,
			wantStderr:   "session not found: missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "kill-session",
				Flags:   map[string]any{"-t": tt.target},
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantStderr != "" {
				if !strings.Contains(resp.Stderr, tt.wantStderr) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
				}
				if len(emitter.Events()) != 0 {
					t.Fatalf("unexpected events emitted on error: %v", emitter.EventNames())
				}
				return
			}

			events := emitter.Events()
			if len(events) != 1 {
				t.Fatalf("events = %v, want exactly one event", emitter.EventNames())
			}
			if events[0].name != tt.wantEvent {
				t.Fatalf("event name = %q, want %q", events[0].name, tt.wantEvent)
			}
			payload, ok := events[0].payload.(map[string]any)
			if !ok {
				t.Fatalf("event payload type = %T, want map[string]any", events[0].payload)
			}
			if got := payload["name"]; got != tt.wantName {
				t.Fatalf("event payload name = %v, want %q", got, tt.wantName)
			}
			if sessions.HasSession("demo") {
				t.Fatal("session demo still exists after successful kill-session")
			}
		})
	}
}

func TestHandleKillSessionCallsOnSessionDestroyed(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var calledSession string
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{
		ShimAvailable: true,
		OnSessionDestroyed: func(sessionName string) {
			calledSession = sessionName
		},
	})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "kill-session",
		Flags:   map[string]any{"-t": "demo"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	if calledSession != "demo" {
		t.Fatalf("OnSessionDestroyed called with %q, want %q", calledSession, "demo")
	}
}

func TestHandleRenameSessionCallsOnSessionRenamed(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	calledOld := ""
	calledNew := ""
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{
		ShimAvailable: true,
		OnSessionRenamed: func(oldName, newName string) {
			calledOld = oldName
			calledNew = newName
		},
	})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "rename-session",
		Flags:   map[string]any{"-t": "demo"},
		Args:    []string{"renamed"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	if calledOld != "demo" || calledNew != "renamed" {
		t.Fatalf("OnSessionRenamed called with (%q, %q), want (%q, %q)", calledOld, calledNew, "demo", "renamed")
	}
}

func TestHandleRenameSession(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(sm *SessionManager) // additional sessions to create
		target       string
		args         []string
		wantExitCode int
		wantStderr   string
		wantEvent    string
		wantOldName  string
		wantNewName  string
	}{
		{
			name:         "success renames session and emits event",
			target:       "demo",
			args:         []string{"newdemo"},
			wantExitCode: 0,
			wantEvent:    "tmux:session-renamed",
			wantOldName:  "demo",
			wantNewName:  "newdemo",
		},
		{
			name:         "session:window target resolves to session name",
			target:       "demo:0",
			args:         []string{"renamed"},
			wantExitCode: 0,
			wantEvent:    "tmux:session-renamed",
			wantOldName:  "demo",
			wantNewName:  "renamed",
		},
		{
			name:         "missing -t returns error",
			target:       "",
			args:         []string{"newdemo"},
			wantExitCode: 1,
			wantStderr:   "rename-session requires -t",
		},
		{
			name:         "missing new-name arg returns error",
			target:       "demo",
			args:         []string{},
			wantExitCode: 1,
			wantStderr:   "rename-session requires new-name argument",
		},
		{
			name:         "empty new-name arg returns error",
			target:       "demo",
			args:         []string{"  "},
			wantExitCode: 1,
			wantStderr:   "rename-session requires new-name argument",
		},
		{
			name:         "session not found returns error",
			target:       "nonexistent",
			args:         []string{"newname"},
			wantExitCode: 1,
			wantStderr:   "session not found",
		},
		{
			name: "duplicate name returns error",
			setup: func(sm *SessionManager) {
				if _, _, err := sm.CreateSession("other", "main", 120, 40); err != nil {
					// test will fail later if this errors
					return
				}
			},
			target:       "demo",
			args:         []string{"other"},
			wantExitCode: 1,
			wantStderr:   "session already exists: other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
			if tt.setup != nil {
				tt.setup(sessions)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "rename-session",
				Flags:   map[string]any{"-t": tt.target},
				Args:    tt.args,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantExitCode == 0 && resp.Stdout != "" {
				t.Errorf("Stdout = %q, want empty on success", resp.Stdout)
			}
			if tt.wantStderr != "" && !strings.Contains(resp.Stderr, tt.wantStderr) {
				t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
			}

			// Verify event emission
			events := emitter.Events()
			if tt.wantEvent == "" {
				if len(events) != 0 {
					t.Fatalf("unexpected events emitted: %v", emitter.EventNames())
				}
				return
			}

			var found bool
			for _, ev := range events {
				if ev.name != tt.wantEvent {
					continue
				}
				found = true
				payload, ok := ev.payload.(map[string]any)
				if !ok {
					t.Fatalf("event payload type = %T, want map[string]any", ev.payload)
				}
				if got := payload["oldName"]; got != tt.wantOldName {
					t.Errorf("event oldName = %v, want %q", got, tt.wantOldName)
				}
				if got := payload["newName"]; got != tt.wantNewName {
					t.Errorf("event newName = %v, want %q", got, tt.wantNewName)
				}
			}
			if !found {
				t.Fatalf("expected event %q not found in %v", tt.wantEvent, emitter.EventNames())
			}
		})
	}
}

func TestHandleShowEnvironment(t *testing.T) {
	tests := []struct {
		name         string
		setupEnv     map[string]string // env vars to set before test
		target       string
		flags        map[string]any
		args         []string
		wantExitCode int
		wantStdout   string
		wantStderr   string
	}{
		{
			name:         "returns sorted env vars for session",
			setupEnv:     map[string]string{"EDITOR": "vim", "SHELL": "/bin/bash", "APP_MODE": "test"},
			target:       "demo",
			flags:        map[string]any{},
			wantExitCode: 0,
			wantStdout:   "APP_MODE=test\nEDITOR=vim\nSHELL=/bin/bash\n",
		},
		{
			name:         "specific variable lookup returns single value",
			setupEnv:     map[string]string{"EDITOR": "vim", "SHELL": "/bin/bash"},
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{"EDITOR"},
			wantExitCode: 0,
			wantStdout:   "EDITOR=vim\n",
		},
		{
			name:         "unknown variable returns exit code 1",
			setupEnv:     map[string]string{"EDITOR": "vim"},
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{"NONEXISTENT"},
			wantExitCode: 1,
			wantStderr:   "unknown variable: NONEXISTENT",
		},
		{
			name:         "global flag -g returns empty stdout",
			target:       "demo",
			flags:        map[string]any{"-g": true},
			wantExitCode: 0,
			wantStdout:   "",
		},
		{
			name:         "global flag -g without target returns empty stdout",
			target:       "",
			flags:        map[string]any{"-g": true},
			wantExitCode: 0,
			wantStdout:   "",
		},
		{
			name:         "session not found returns error",
			target:       "nonexistent",
			flags:        map[string]any{},
			wantExitCode: 1,
			wantStderr:   "session not found",
		},
		{
			name:         "empty session env returns empty stdout",
			target:       "demo",
			flags:        map[string]any{},
			wantExitCode: 0,
			wantStdout:   "",
		},
		{
			name:         "empty target without -g returns empty stdout",
			target:       "",
			flags:        map[string]any{},
			wantExitCode: 0,
			wantStdout:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			// Set up environment variables
			for k, v := range tt.setupEnv {
				if err := sessions.SetSessionEnv("demo", k, v); err != nil {
					t.Fatalf("SetSessionEnv(%q, %q) error = %v", k, v, err)
				}
			}

			flags := tt.flags
			if flags == nil {
				flags = map[string]any{}
			}
			flags["-t"] = tt.target

			resp := router.Execute(ipc.TmuxRequest{
				Command: "show-environment",
				Flags:   flags,
				Args:    tt.args,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q, stdout=%q",
					resp.ExitCode, tt.wantExitCode, resp.Stderr, resp.Stdout)
			}
			if tt.wantStderr != "" && !strings.Contains(resp.Stderr, tt.wantStderr) {
				t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
			}
			if tt.wantExitCode == 0 && resp.Stdout != tt.wantStdout {
				t.Fatalf("Stdout = %q, want %q", resp.Stdout, tt.wantStdout)
			}

			// For success cases with multiple env vars, verify sorting
			if tt.wantExitCode == 0 && len(tt.setupEnv) > 1 && tt.args == nil {
				lines := strings.Split(strings.TrimRight(resp.Stdout, "\n"), "\n")
				if len(lines) > 0 && lines[0] != "" {
					keys := make([]string, 0, len(lines))
					for _, line := range lines {
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							keys = append(keys, parts[0])
						}
					}
					if !sort.StringsAreSorted(keys) {
						t.Errorf("env vars not sorted: keys = %v", keys)
					}
				}
			}
		})
	}
}

func TestHandleSetEnvironment(t *testing.T) {
	tests := []struct {
		name         string
		setupEnv     map[string]string // env vars to pre-set before test
		target       string
		flags        map[string]any
		args         []string
		wantExitCode int
		wantStderr   string
		verifyEnv    func(t *testing.T, sm *SessionManager)
	}{
		{
			name:         "set a variable succeeds",
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{"MY_VAR", "my_value"},
			wantExitCode: 0,
			verifyEnv: func(t *testing.T, sm *SessionManager) {
				t.Helper()
				env, err := sm.GetSessionEnv("demo")
				if err != nil {
					t.Fatalf("GetSessionEnv() error = %v", err)
				}
				if got, ok := env["MY_VAR"]; !ok || got != "my_value" {
					t.Fatalf("MY_VAR = %q (exists=%v), want %q", got, ok, "my_value")
				}
			},
		},
		{
			name:         "overwrite existing variable",
			setupEnv:     map[string]string{"MY_VAR": "old_value"},
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{"MY_VAR", "new_value"},
			wantExitCode: 0,
			verifyEnv: func(t *testing.T, sm *SessionManager) {
				t.Helper()
				env, err := sm.GetSessionEnv("demo")
				if err != nil {
					t.Fatalf("GetSessionEnv() error = %v", err)
				}
				if got := env["MY_VAR"]; got != "new_value" {
					t.Fatalf("MY_VAR = %q, want %q", got, "new_value")
				}
			},
		},
		{
			name:         "unset a variable removes it",
			setupEnv:     map[string]string{"REMOVE_ME": "value"},
			target:       "demo",
			flags:        map[string]any{"-u": true},
			args:         []string{"REMOVE_ME"},
			wantExitCode: 0,
			verifyEnv: func(t *testing.T, sm *SessionManager) {
				t.Helper()
				env, err := sm.GetSessionEnv("demo")
				if err != nil {
					t.Fatalf("GetSessionEnv() error = %v", err)
				}
				if _, ok := env["REMOVE_ME"]; ok {
					t.Fatal("REMOVE_ME still present after unset")
				}
			},
		},
		{
			name:         "global flag -g returns success as no-op",
			target:       "demo",
			flags:        map[string]any{"-g": true},
			args:         []string{"SOME_VAR", "some_value"},
			wantExitCode: 0,
			verifyEnv: func(t *testing.T, sm *SessionManager) {
				t.Helper()
				env, err := sm.GetSessionEnv("demo")
				if err != nil {
					t.Fatalf("GetSessionEnv() error = %v", err)
				}
				if _, ok := env["SOME_VAR"]; ok {
					t.Fatal("SOME_VAR should not be set when -g is used")
				}
			},
		},
		{
			name:         "missing -t returns error",
			target:       "",
			flags:        map[string]any{},
			args:         []string{"MY_VAR", "value"},
			wantExitCode: 1,
			wantStderr:   "set-environment requires -t",
		},
		{
			name:         "missing variable name returns error",
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{},
			wantExitCode: 1,
			wantStderr:   "set-environment requires variable name",
		},
		{
			name:         "empty variable name returns error",
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{"  "},
			wantExitCode: 1,
			wantStderr:   "set-environment requires variable name",
		},
		{
			name:         "missing value without -u returns error",
			target:       "demo",
			flags:        map[string]any{},
			args:         []string{"MY_VAR"},
			wantExitCode: 1,
			wantStderr:   "set-environment requires variable value",
		},
		{
			name:         "session not found returns error",
			target:       "nonexistent",
			flags:        map[string]any{},
			args:         []string{"MY_VAR", "value"},
			wantExitCode: 1,
			wantStderr:   "session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			// Pre-set environment variables
			for k, v := range tt.setupEnv {
				if err := sessions.SetSessionEnv("demo", k, v); err != nil {
					t.Fatalf("SetSessionEnv(%q, %q) error = %v", k, v, err)
				}
			}

			flags := tt.flags
			if flags == nil {
				flags = map[string]any{}
			}
			flags["-t"] = tt.target

			resp := router.Execute(ipc.TmuxRequest{
				Command: "set-environment",
				Flags:   flags,
				Args:    tt.args,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantExitCode == 0 && resp.Stdout != "" {
				t.Errorf("Stdout = %q, want empty on success", resp.Stdout)
			}
			if tt.wantStderr != "" && !strings.Contains(resp.Stderr, tt.wantStderr) {
				t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
			}

			if tt.verifyEnv != nil {
				tt.verifyEnv(t, sessions)
			}
		})
	}
}

// TestHandleSetEnvironmentBlockedKeyFiltering verifies that blocked system keys
// (PATH, SYSTEMROOT, etc.) stored via set-environment are filtered at the pane
// creation layer. Non-blocked keys must pass through to the pane environment.
//
// Defense-in-depth: set-environment stores all keys (session env is unrestricted),
// but buildPaneEnv / buildPaneEnvForSession / mergeEnvironment filter blocked keys
// before they reach the process environment. This test validates the full pipeline.
func TestHandleSetEnvironmentBlockedKeyFiltering(t *testing.T) {
	tests := []struct {
		name      string
		envKey    string
		envValue  string
		isBlocked bool
	}{
		{
			name:      "PATH is blocked",
			envKey:    "PATH",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "SYSTEMROOT is blocked",
			envKey:    "SYSTEMROOT",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "COMSPEC is blocked",
			envKey:    "COMSPEC",
			envValue:  `C:\evil.exe`,
			isBlocked: true,
		},
		{
			name:      "WINDIR is blocked",
			envKey:    "WINDIR",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "APPDATA is blocked",
			envKey:    "APPDATA",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "LOCALAPPDATA is blocked",
			envKey:    "LOCALAPPDATA",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "TEMP is blocked",
			envKey:    "TEMP",
			envValue:  `C:\attacker\temp`,
			isBlocked: true,
		},
		{
			name:      "TMP is blocked",
			envKey:    "TMP",
			envValue:  `C:\attacker\tmp`,
			isBlocked: true,
		},
		{
			name:      "USERPROFILE is blocked",
			envKey:    "USERPROFILE",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "PATHEXT is blocked",
			envKey:    "PATHEXT",
			envValue:  ".evil",
			isBlocked: true,
		},
		{
			name:      "SYSTEMDRIVE is blocked",
			envKey:    "SYSTEMDRIVE",
			envValue:  "X:",
			isBlocked: true,
		},
		{
			name:      "PSMODULEPATH is blocked",
			envKey:    "PSMODULEPATH",
			envValue:  `C:\attacker\modules`,
			isBlocked: true,
		},
		{
			name:      "case-insensitive: lowercase path is blocked",
			envKey:    "path",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "case-insensitive: mixed case ComSpec is blocked",
			envKey:    "ComSpec",
			envValue:  `C:\evil.exe`,
			isBlocked: true,
		},
		{
			name:      "MY_CUSTOM_VAR is not blocked",
			envKey:    "MY_CUSTOM_VAR",
			envValue:  "custom_value",
			isBlocked: false,
		},
		{
			name:      "CLAUDE_CODE_EFFORT_LEVEL is not blocked",
			envKey:    "CLAUDE_CODE_EFFORT_LEVEL",
			envValue:  "high",
			isBlocked: false,
		},
		{
			name:      "EDITOR is not blocked",
			envKey:    "EDITOR",
			envValue:  "vim",
			isBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{ShimAvailable: true})

			if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			// Step 1: set-environment stores the key (no filtering at this layer).
			resp := router.Execute(ipc.TmuxRequest{
				Command: "set-environment",
				Flags:   map[string]any{"-t": "demo"},
				Args:    []string{tt.envKey, tt.envValue},
			})
			if resp.ExitCode != 0 {
				t.Fatalf("set-environment failed: %s", resp.Stderr)
			}

			// Step 2: verify the key was stored in session env (no filtering at storage).
			env, err := sessions.GetSessionEnv("demo")
			if err != nil {
				t.Fatalf("GetSessionEnv() error = %v", err)
			}
			if _, stored := env[tt.envKey]; !stored {
				t.Fatalf("session env should store key %q (filtering is at pane creation, not storage)", tt.envKey)
			}

			// Step 3: verify isBlockedEnvironmentKey correctly identifies the key.
			if got := isBlockedEnvironmentKey(tt.envKey); got != tt.isBlocked {
				t.Errorf("isBlockedEnvironmentKey(%q) = %v, want %v", tt.envKey, got, tt.isBlocked)
			}

			// Step 4: verify that buildPaneEnv filters blocked keys from pane environment.
			paneEnv := router.buildPaneEnv(env, 0, 0)
			if tt.isBlocked {
				if _, exists := paneEnv[tt.envKey]; exists {
					t.Errorf("blocked key %q should NOT appear in pane env", tt.envKey)
				}
			} else {
				if paneEnv[tt.envKey] != tt.envValue {
					t.Errorf("non-blocked key %q should appear in pane env with value %q, got %q",
						tt.envKey, tt.envValue, paneEnv[tt.envKey])
				}
			}
		})
	}
}

// TestBuildPaneEnvForSessionBlockedKeyFiltering verifies that buildPaneEnvForSession
// filters blocked system keys arriving via inheritedEnv (Layer 2) and shimEnv (Layer 4).
//
// Design note: pane_env (Layer 3) and claude_env (Layer 1) are admin-controlled
// config and intentionally NOT filtered by isBlockedEnvironmentKey. Blocked-key
// enforcement for those layers is deferred to downstream mergeEnvironment /
// sanitizeCustomEnvironmentEntry. This test focuses on the inheritedEnv path
// which carries user-supplied session env (set-environment).
func TestBuildPaneEnvForSessionBlockedKeyFiltering(t *testing.T) {
	tests := []struct {
		name      string
		envKey    string
		envValue  string
		isBlocked bool
	}{
		{
			name:      "PATH via inheritedEnv is blocked",
			envKey:    "PATH",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "COMSPEC via inheritedEnv is blocked",
			envKey:    "COMSPEC",
			envValue:  `C:\evil.exe`,
			isBlocked: true,
		},
		{
			name:      "case-insensitive: mixed case ComSpec via inheritedEnv is blocked",
			envKey:    "ComSpec",
			envValue:  `C:\evil.exe`,
			isBlocked: true,
		},
		{
			name:      "SYSTEMROOT via inheritedEnv is blocked",
			envKey:    "SYSTEMROOT",
			envValue:  `C:\attacker`,
			isBlocked: true,
		},
		{
			name:      "MY_CUSTOM_VAR via inheritedEnv is not blocked",
			envKey:    "MY_CUSTOM_VAR",
			envValue:  "custom_value",
			isBlocked: false,
		},
		{
			name:      "EDITOR via inheritedEnv is not blocked",
			envKey:    "EDITOR",
			envValue:  "vim",
			isBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			// No PaneEnv configured: this test targets inheritedEnv (Layer 2) filtering only.
			router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{
				ShimAvailable: true,
			})

			if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			// Store the key in session env to simulate inheritedEnv source.
			if err := sessions.SetSessionEnv("demo", tt.envKey, tt.envValue); err != nil {
				t.Fatalf("SetSessionEnv(%q, %q) error = %v", tt.envKey, tt.envValue, err)
			}
			sessionEnv, err := sessions.GetSessionEnv("demo")
			if err != nil {
				t.Fatalf("GetSessionEnv() error = %v", err)
			}

			// Call buildPaneEnvForSession with inheritedEnv containing the key.
			// usePaneEnv=false to isolate Layer 2 filtering without Layer 3 interference.
			paneEnv := router.buildPaneEnvForSession(
				sessionEnv, // inheritedEnv (Layer 2: filtered by isBlockedEnvironmentKey)
				nil,        // shimEnv
				0, 0,       // sessionID, paneID
				false, // useClaudeEnv
				false, // usePaneEnv
			)

			if tt.isBlocked {
				if _, exists := paneEnv[tt.envKey]; exists {
					t.Errorf("blocked key %q should NOT appear in buildPaneEnvForSession result", tt.envKey)
				}
			} else {
				if paneEnv[tt.envKey] != tt.envValue {
					t.Errorf("non-blocked key %q should appear in buildPaneEnvForSession result with value %q, got %q",
						tt.envKey, tt.envValue, paneEnv[tt.envKey])
				}
			}
		})
	}
}
