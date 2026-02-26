package tmux

import (
	"errors"
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestHandleActivateWindow(t *testing.T) {
	tests := []struct {
		name          string
		wantExitCode  int
		wantStdout    string
		wantEventName string
	}{
		{
			name:          "returns exit code 0 and emits event",
			wantExitCode:  0,
			wantStdout:    "ok\n",
			wantEventName: "app:activate-window",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			router := NewCommandRouter(NewSessionManager(), emitter, RouterOptions{
				DefaultShell: "cmd.exe",
			})

			resp := router.Execute(ipc.TmuxRequest{Command: "activate-window"})

			if resp.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", resp.ExitCode, tt.wantExitCode)
			}
			if resp.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", resp.Stdout, tt.wantStdout)
			}

			events := emitter.Events()
			if len(events) != 1 {
				t.Fatalf("emitted %d events, want 1", len(events))
			}
			if events[0].name != tt.wantEventName {
				t.Errorf("event name = %q, want %q", events[0].name, tt.wantEventName)
			}
			if events[0].payload != nil {
				t.Errorf("event payload = %#v, want nil", events[0].payload)
			}
		})
	}
}

func TestHandleActivateWindowWithNilEmitter(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		DefaultShell: "cmd.exe",
	})

	resp := router.Execute(ipc.TmuxRequest{Command: "activate-window"})

	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", resp.ExitCode)
	}
	if resp.Stdout != "ok\n" {
		t.Fatalf("Stdout = %q, want %q", resp.Stdout, "ok\n")
	}
}

func TestHandleListWindows(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, sessions *SessionManager)
		flags        map[string]any
		callerPane   string
		wantExitCode int
		wantStderr   string
		// wantLines lists substrings that must each appear in distinct output lines.
		wantLines []string
		// wantLineCount, when > 0, asserts the exact number of output lines.
		wantLineCount int
	}{
		{
			name: "single session with one window",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:         map[string]any{"-t": "demo"},
			wantExitCode:  0,
			wantLines:     []string{"main"},
			wantLineCount: 1,
		},

		{
			name: "all sessions with -a flag",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("alpha", "win-a", 120, 40); err != nil {
					t.Fatalf("CreateSession(alpha) error = %v", err)
				}
				if _, _, err := sessions.CreateSession("beta", "win-b", 120, 40); err != nil {
					t.Fatalf("CreateSession(beta) error = %v", err)
				}
			},
			flags:         map[string]any{"-a": true},
			wantExitCode:  0,
			wantLines:     []string{"win-a", "win-b"},
			wantLineCount: 2,
		},
		{
			name: "no target without -a lists caller session windows",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("alpha", "win-a", 120, 40); err != nil {
					t.Fatalf("CreateSession(alpha) error = %v", err)
				}
				if _, _, err := sessions.CreateSession("beta", "win-b", 120, 40); err != nil {
					t.Fatalf("CreateSession(beta) error = %v", err)
				}
			},
			flags:         map[string]any{},
			callerPane:    "%1",
			wantExitCode:  0,
			wantLines:     []string{"win-b"},
			wantLineCount: 1,
		},

		{
			name: "no target and no -a uses current session",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("only", "sole", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:         map[string]any{},
			wantExitCode:  0,
			wantLines:     []string{"sole"},
			wantLineCount: 1,
		},
		{
			name: "session not found returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				// No sessions created.
			},
			flags:        map[string]any{"-t": "nonexistent"},
			wantExitCode: 1,
			wantStderr:   "session not found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			tt.setup(t, sessions)

			resp := router.Execute(ipc.TmuxRequest{
				Command:    "list-windows",
				Flags:      tt.flags,
				CallerPane: tt.callerPane,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.wantStderr != "" {
				if !strings.Contains(resp.Stderr, tt.wantStderr) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
				}
				return
			}

			// Validate output lines.
			lines := splitNonEmpty(resp.Stdout)
			if tt.wantLineCount > 0 && len(lines) != tt.wantLineCount {
				t.Fatalf("output line count = %d, want %d, stdout=%q", len(lines), tt.wantLineCount, resp.Stdout)
			}
			for _, want := range tt.wantLines {
				if !containsInAnyLine(lines, want) {
					t.Errorf("output missing substring %q, stdout=%q", want, resp.Stdout)
				}
			}
		})
	}
}

func TestHandleRenameWindow(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, sessions *SessionManager)
		flags          map[string]any
		args           []string
		wantExitCode   int
		wantStderr     string
		wantEventName  string
		wantWindowName string
	}{
		{
			name: "success renames window and emits event",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:          map[string]any{"-t": "demo:0"},
			args:           []string{"renamed"},
			wantExitCode:   0,
			wantEventName:  "tmux:window-renamed",
			wantWindowName: "renamed",
		},
		{
			name: "missing new-name argument returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "demo:0"},
			args:         []string{},
			wantExitCode: 1,
			wantStderr:   "rename-window requires new-name argument",
		},
		{
			name: "whitespace-only new-name returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "demo:0"},
			args:         []string{"   "},
			wantExitCode: 1,
			wantStderr:   "rename-window requires new-name argument",
		},
		{
			name: "target not found returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "nonexistent:0"},
			args:         []string{"newname"},
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
			tt.setup(t, sessions)

			resp := router.Execute(ipc.TmuxRequest{
				Command: "rename-window",
				Flags:   tt.flags,
				Args:    tt.args,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.wantStderr != "" {
				if !strings.Contains(resp.Stderr, tt.wantStderr) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
				}
				return
			}

			// Verify event was emitted.
			events := emitter.Events()
			if len(events) != 1 {
				t.Fatalf("emitted %d events, want 1", len(events))
			}
			if events[0].name != tt.wantEventName {
				t.Fatalf("event name = %q, want %q", events[0].name, tt.wantEventName)
			}
			payload, ok := events[0].payload.(map[string]any)
			if !ok {
				t.Fatalf("event payload type = %T, want map[string]any", events[0].payload)
			}
			if got := mustString(payload["windowName"]); got != tt.wantWindowName {
				t.Fatalf("event payload windowName = %q, want %q", got, tt.wantWindowName)
			}

			// Verify the window name actually changed in the session manager.
			session, ok := sessions.GetSession("demo")
			if !ok {
				t.Fatal("session 'demo' not found after rename")
			}
			if len(session.Windows) == 0 {
				t.Fatal("session has no windows after rename")
			}
			if session.Windows[0].Name != tt.wantWindowName {
				t.Fatalf("window name = %q, want %q", session.Windows[0].Name, tt.wantWindowName)
			}
		})
	}
}

func TestHandleKillWindow(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(t *testing.T, sessions *SessionManager)
		flags            map[string]any
		wantExitCode     int
		wantStderr       string
		wantSessionAlive bool
		wantWindowID     int
		wantLayoutEvent  bool
		wantWindowNames  []string
	}{
		{
			name: "kill last window destroys session",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:            map[string]any{"-t": "demo:0"},
			wantExitCode:     0,
			wantSessionAlive: false,
		},
		{
			name: "kill one window in multi-window session keeps session alive",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
				// Inject a second window to validate the non-destroying kill path.
				injectTestWindow(t, sessions, "demo", "second")
			},
			flags:            map[string]any{"-t": "demo:0"},
			wantExitCode:     0,
			wantSessionAlive: true,
			wantWindowID:     0,
			wantLayoutEvent:  true,
			wantWindowNames:  []string{"second"},
		},

		{
			name: "target not found returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "nonexistent:0"},
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
			tt.setup(t, sessions)

			resp := router.Execute(ipc.TmuxRequest{
				Command: "kill-window",
				Flags:   tt.flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.wantStderr != "" {
				if !strings.Contains(resp.Stderr, tt.wantStderr) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
				}
				if len(emitter.Events()) != 0 {
					t.Fatalf("unexpected events on error: %v", emitter.EventNames())
				}
				return
			}

			destroyedCount := 0
			windowDestroyedCount := 0
			layoutChangedCount := 0
			for _, event := range emitter.Events() {
				payload, ok := event.payload.(map[string]any)
				if !ok {
					t.Fatalf("event %q payload type = %T, want map[string]any", event.name, event.payload)
				}
				switch event.name {
				case "tmux:session-destroyed":
					destroyedCount++
					if got := mustString(payload["name"]); got != "demo" {
						t.Fatalf("session-destroyed name = %q, want %q", got, "demo")
					}
				case "tmux:window-destroyed":
					windowDestroyedCount++
					if got := mustString(payload["sessionName"]); got != "demo" {
						t.Fatalf("window-destroyed sessionName = %q, want %q", got, "demo")
					}
					if got := mustInt(payload["windowId"], -1); got != tt.wantWindowID {
						t.Fatalf("window-destroyed windowId = %d, want %d", got, tt.wantWindowID)
					}
				case "tmux:layout-changed":
					layoutChangedCount++
					if got := mustString(payload["sessionName"]); got != "demo" {
						t.Fatalf("layout-changed sessionName = %q, want %q", got, "demo")
					}
					if payload["layoutTree"] == nil {
						t.Fatal("layout-changed layoutTree is nil")
					}
				}
			}

			if tt.wantSessionAlive {
				if destroyedCount != 0 {
					t.Fatalf("session-destroyed count = %d, want 0", destroyedCount)
				}
				if windowDestroyedCount != 1 {
					t.Fatalf("window-destroyed count = %d, want 1", windowDestroyedCount)
				}
				if tt.wantLayoutEvent && layoutChangedCount != 1 {
					t.Fatalf("layout-changed count = %d, want 1", layoutChangedCount)
				}
			} else {
				if destroyedCount != 1 {
					t.Fatalf("session-destroyed count = %d, want 1", destroyedCount)
				}
				if windowDestroyedCount != 0 {
					t.Fatalf("window-destroyed count = %d, want 0", windowDestroyedCount)
				}
				if layoutChangedCount != 0 {
					t.Fatalf("layout-changed count = %d, want 0", layoutChangedCount)
				}
			}

			if got := sessions.HasSession("demo"); got != tt.wantSessionAlive {
				t.Fatalf("HasSession(demo) = %v, want %v", got, tt.wantSessionAlive)
			}
			if tt.wantSessionAlive {
				session, ok := sessions.GetSession("demo")
				if !ok {
					t.Fatal("GetSession(demo) failed")
				}
				if len(session.Windows) != len(tt.wantWindowNames) {
					t.Fatalf("window count = %d, want %d", len(session.Windows), len(tt.wantWindowNames))
				}
				for i, wantName := range tt.wantWindowNames {
					if got := session.Windows[i].Name; got != wantName {
						t.Fatalf("window[%d].Name = %q, want %q", i, got, wantName)
					}
				}
			}
		})
	}
}

func TestHandleNewWindow(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(t *testing.T, sessions *SessionManager)
		flags            map[string]any
		attachErr        error
		forceSnapRefetch bool
		wantExitCode     int
		wantStderr       string
		wantNewSession   bool
		wantRollback     bool
		wantEventName    string
		wantAgentTeam    bool   // if true, verify child session IsAgentTeam == true
		wantUseClaudeEnv *bool  // non-nil: verify child UseClaudeEnv matches
		wantUsePaneEnv   *bool  // non-nil: verify child UsePaneEnv matches
		wantOutputPrefix string // non-empty: verify Stdout has this prefix
	}{
		{
			name: "creates new session and emits session-created event",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:          map[string]any{"-t": "parent", "-n": "child"},
			wantExitCode:   0,
			wantNewSession: true,
			wantEventName:  "tmux:session-created",
		},
		{
			name: "missing -t returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-n": "child"},
			wantExitCode: 1,
			wantStderr:   "new-window requires -t with parent session name",
		},
		{
			name: "missing -n returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "parent"},
			wantExitCode: 1,
			wantStderr:   "new-window requires -n flag",
		},
		{
			name: "parent session not found returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				// No sessions created.
			},
			flags:        map[string]any{"-t": "missing", "-n": "child"},
			wantExitCode: 1,
			wantStderr:   "parent session not found: missing",
		},
		{
			name: "duplicate session name returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession(parent) error = %v", err)
				}
				if _, _, err := sessions.CreateSession("child", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession(child) error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "parent", "-n": "child"},
			wantExitCode: 1,
			wantStderr:   "session already exists: child",
		},
		{
			name: "attach failure rolls back session creation",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "parent", "-n": "rollback-child"},
			attachErr:    errors.New("attach failed"),
			wantExitCode: 1,
			wantStderr:   "attach failed",
			wantRollback: true,
		},
		{
			name: "snapshot refetch failure rolls back session creation",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:            map[string]any{"-t": "parent", "-n": "snap-missing-child"},
			forceSnapRefetch: true,
			wantExitCode:     1,
			wantStderr:       "session disappeared during setup: snap-missing-child",
			wantRollback:     true,
		},
		{
			name: "inherits IsAgentTeam from parent",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
				if err := sessions.SetAgentTeam("parent", true); err != nil {
					t.Fatalf("SetAgentTeam() error = %v", err)
				}
			},
			flags:          map[string]any{"-t": "parent", "-n": "agent-child"},
			wantExitCode:   0,
			wantNewSession: true,
			wantEventName:  "tmux:session-created",
			wantAgentTeam:  true,
		},
		{
			name: "inherits UseClaudeEnv from parent",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
				if err := sessions.SetUseClaudeEnv("parent", true); err != nil {
					t.Fatalf("SetUseClaudeEnv() error = %v", err)
				}
			},
			flags:            map[string]any{"-t": "parent", "-n": "claude-child"},
			wantExitCode:     0,
			wantNewSession:   true,
			wantEventName:    "tmux:session-created",
			wantUseClaudeEnv: new(true),
		},
		{
			name: "inherits UsePaneEnv from parent",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
				if err := sessions.SetUsePaneEnv("parent", false); err != nil {
					t.Fatalf("SetUsePaneEnv() error = %v", err)
				}
			},
			flags:          map[string]any{"-t": "parent", "-n": "pane-child"},
			wantExitCode:   0,
			wantNewSession: true,
			wantEventName:  "tmux:session-created",
			wantUsePaneEnv: new(false),
		},
		{
			name: "-d flag does not change active pane",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:          map[string]any{"-t": "parent", "-n": "detach-child", "-d": true},
			wantExitCode:   0,
			wantNewSession: true,
			wantEventName:  "tmux:session-created",
		},
		{
			name: "-P flag returns formatted output",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("parent", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:            map[string]any{"-t": "parent", "-n": "print-child", "-P": true, "-F": "#{session_name}:#{window_index}"},
			wantExitCode:     0,
			wantNewSession:   true,
			wantEventName:    "tmux:session-created",
			wantOutputPrefix: "print-child:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			tt.setup(t, sessions)
			if tt.attachErr != nil {
				router.attachTerminalFn = func(*TmuxPane, string, map[string]string, *TmuxPane) error {
					return tt.attachErr
				}
			} else {
				router.attachTerminalFn = func(*TmuxPane, string, map[string]string, *TmuxPane) error {
					return nil
				}
			}
			if tt.forceSnapRefetch {
				childName := mustString(tt.flags["-n"])
				router.getSessionForNewWindowFn = func(name string) (*TmuxSession, bool) {
					if name == childName {
						return nil, false
					}
					return sessions.GetSession(name)
				}
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "new-window",
				Flags:   tt.flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantStderr != "" {
				if !strings.Contains(resp.Stderr, tt.wantStderr) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
				}
			}

			if tt.wantNewSession {
				newSessionName := mustString(tt.flags["-n"])
				childSession, ok := sessions.GetSession(newSessionName)
				if !ok {
					t.Fatalf("new session %q not found", newSessionName)
				}
				if len(childSession.Windows) != 1 {
					t.Fatalf("new session window count = %d, want 1", len(childSession.Windows))
				}

				// Verify event
				events := emitter.Events()
				found := false
				for _, ev := range events {
					if ev.name == tt.wantEventName {
						found = true
						payload, ok := ev.payload.(map[string]any)
						if !ok {
							t.Fatalf("event payload type = %T, want map[string]any", ev.payload)
						}
						if got := mustString(payload["name"]); got != newSessionName {
							t.Fatalf("event name = %q, want %q", got, newSessionName)
						}
						break
					}
				}
				if !found {
					t.Fatalf("event %q not found in %v", tt.wantEventName, emitter.EventNames())
				}
			}

			// For rollback paths: verify child session was removed and no create event leaked.
			if tt.wantRollback {
				childName := mustString(tt.flags["-n"])
				if sessions.HasSession(childName) {
					t.Fatalf("child session %q still exists after rollback", childName)
				}
				for _, ev := range emitter.Events() {
					if ev.name == "tmux:session-created" {
						t.Fatalf("unexpected session-created event emitted during rollback path")
					}
				}
			}

			if tt.wantAgentTeam {
				childSession, ok := sessions.GetSession(mustString(tt.flags["-n"]))
				if !ok {
					t.Fatal("child session not found")
				}
				if !childSession.IsAgentTeam {
					t.Fatal("child session IsAgentTeam = false, want true")
				}
			}

			if tt.wantUseClaudeEnv != nil {
				childSession, ok := sessions.GetSession(mustString(tt.flags["-n"]))
				if !ok {
					t.Fatal("child session not found")
				}
				if childSession.UseClaudeEnv == nil || *childSession.UseClaudeEnv != *tt.wantUseClaudeEnv {
					t.Fatalf("child session UseClaudeEnv = %v, want %v", childSession.UseClaudeEnv, tt.wantUseClaudeEnv)
				}
			}

			if tt.wantUsePaneEnv != nil {
				childSession, ok := sessions.GetSession(mustString(tt.flags["-n"]))
				if !ok {
					t.Fatal("child session not found")
				}
				if childSession.UsePaneEnv == nil || *childSession.UsePaneEnv != *tt.wantUsePaneEnv {
					t.Fatalf("child session UsePaneEnv = %v, want %v", childSession.UsePaneEnv, tt.wantUsePaneEnv)
				}
			}

			if tt.wantOutputPrefix != "" {
				if !strings.HasPrefix(resp.Stdout, tt.wantOutputPrefix) {
					t.Fatalf("Stdout = %q, want prefix %q", resp.Stdout, tt.wantOutputPrefix)
				}
				if !strings.HasSuffix(resp.Stdout, "\n") {
					t.Fatalf("Stdout = %q, want trailing newline", resp.Stdout)
				}
			}
		})
	}
}

func TestHandleSelectWindow(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(t *testing.T, sessions *SessionManager)
		flags              map[string]any
		wantExitCode       int
		wantStderr         string
		wantPaneID         string
		wantActiveWindowID int
	}{

		{
			name: "select first window by target",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:              map[string]any{"-t": "demo:0"},
			wantExitCode:       0,
			wantPaneID:         "%0",
			wantActiveWindowID: 0,
		},
		{
			name: "target not found returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:        map[string]any{"-t": "nonexistent:0"},
			wantExitCode: 1,
			wantStderr:   "session not found",
			// wantActiveWindowID is not checked on error path (sentinel -1 is not meaningful).
			wantActiveWindowID: -1,
		},
		{
			name: "missing target returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				// No sessions needed.
			},
			flags:        map[string]any{},
			wantExitCode: 1,
			wantStderr:   "missing required flag: -t",
			// wantActiveWindowID is not checked on error path (sentinel -1 is not meaningful).
			wantActiveWindowID: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			tt.setup(t, sessions)

			resp := router.Execute(ipc.TmuxRequest{
				Command: "select-window",
				Flags:   tt.flags,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.wantStderr != "" {
				if !strings.Contains(resp.Stderr, tt.wantStderr) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
				}
				return
			}

			// Verify event was emitted.
			events := emitter.Events()
			if len(events) != 1 {
				t.Fatalf("emitted %d events, want 1, events=%v", len(events), emitter.EventNames())
			}
			if events[0].name != "tmux:pane-focused" {
				t.Fatalf("event name = %q, want %q", events[0].name, "tmux:pane-focused")
			}
			payload, ok := events[0].payload.(map[string]any)
			if !ok {
				t.Fatalf("event payload type = %T, want map[string]any", events[0].payload)
			}
			if got := mustString(payload["sessionName"]); got != "demo" {
				t.Fatalf("event payload sessionName = %q, want %q", got, "demo")
			}
			if got := mustString(payload["paneId"]); got != tt.wantPaneID {
				t.Fatalf("event payload paneId = %q, want %q", got, tt.wantPaneID)
			}

			session, ok := sessions.GetSession("demo")
			if !ok {
				t.Fatal("session demo not found")
			}
			if session.ActiveWindowID != tt.wantActiveWindowID {
				t.Fatalf("ActiveWindowID = %d, want %d", session.ActiveWindowID, tt.wantActiveWindowID)
			}
		})
	}
}

func TestResolveWindowFromRequest(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T, sessions *SessionManager)
		flags           map[string]any
		wantSessionName string
		wantWindowIdx   int
		wantErr         string
	}{
		{
			name: "resolve session:window target",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:           map[string]any{"-t": "demo:0"},
			wantSessionName: "demo",
			wantWindowIdx:   0,
		},
		{
			name: "resolve pane-id target",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:           map[string]any{"-t": "%0"},
			wantSessionName: "demo",
			wantWindowIdx:   0,
		},
		{
			name: "resolve session:window.pane target",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:           map[string]any{"-t": "demo:0.0"},
			wantSessionName: "demo",
			wantWindowIdx:   0,
		},

		{
			name: "missing -t flag returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:   map[string]any{},
			wantErr: "missing required flag: -t",
		},
		{
			name: "session not found returns error",
			setup: func(t *testing.T, sessions *SessionManager) {
				if _, _, err := sessions.CreateSession("demo", "main", 120, 40); err != nil {
					t.Fatalf("CreateSession() error = %v", err)
				}
			},
			flags:   map[string]any{"-t": "nonexistent:0"},
			wantErr: "session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})
			tt.setup(t, sessions)

			sessionName, windowIdx, err := router.resolveWindowFromRequest(ipc.TmuxRequest{
				Flags: tt.flags,
			})

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sessionName != tt.wantSessionName {
				t.Fatalf("sessionName = %q, want %q", sessionName, tt.wantSessionName)
			}
			if windowIdx != tt.wantWindowIdx {
				t.Fatalf("windowIdx = %d, want %d", windowIdx, tt.wantWindowIdx)
			}
		})
	}
}

// --- test helpers ---

// splitNonEmpty splits text by newlines and returns only non-empty lines.
func splitNonEmpty(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// containsInAnyLine reports whether any line contains the given substring.
func containsInAnyLine(lines []string, sub string) bool {
	for _, line := range lines {
		if strings.Contains(line, sub) {
			return true
		}
	}
	return false
}
