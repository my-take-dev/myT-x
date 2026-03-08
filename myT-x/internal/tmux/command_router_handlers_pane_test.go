package tmux

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/ipc"
	"myT-x/internal/terminal"
)

func TestSplitWindowWorkDirFallback(t *testing.T) {
	type rootPathMode int
	const (
		rootPathUnset rootPathMode = iota
		rootPathMissing
		rootPathWhitespace
	)

	type workDirMode int
	const (
		workDirOmitted workDirMode = iota
		workDirEmpty
		workDirWhitespace
		workDirValid
	)

	tests := []struct {
		name         string
		rootPath     rootPathMode
		workDir      workDirMode
		wantExitCode int
		wantPaneOne  bool
	}{
		{
			name:         "omitted workDir and unset session root fall back to host cwd",
			rootPath:     rootPathUnset,
			workDir:      workDirOmitted,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
		{
			name:         "omitted workDir falls back to session workDir",
			rootPath:     rootPathMissing,
			workDir:      workDirOmitted,
			wantExitCode: 1,
			wantPaneOne:  false,
		},
		{
			name:         "whitespace workDir falls back to session workDir",
			rootPath:     rootPathMissing,
			workDir:      workDirWhitespace,
			wantExitCode: 1,
			wantPaneOne:  false,
		},
		{
			name:         "explicit workDir is not overwritten by fallback",
			rootPath:     rootPathMissing,
			workDir:      workDirValid,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
		{
			name:         "empty workDir and empty session fallback uses host cwd",
			rootPath:     rootPathWhitespace,
			workDir:      workDirEmpty,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
		{
			name:         "both rootPath and worktreePath empty falls back to host cwd",
			rootPath:     rootPathUnset,
			workDir:      workDirEmpty,
			wantExitCode: 0,
			wantPaneOne:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			missingDir := filepath.Join(t.TempDir(), "missing-workdir")
			validDir := os.TempDir()

			switch tt.rootPath {
			case rootPathMissing:
				if err := sessions.SetRootPath("demo", missingDir); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			case rootPathWhitespace:
				if err := sessions.SetRootPath("demo", "   "); err != nil {
					t.Fatalf("SetRootPath() error = %v", err)
				}
			case rootPathUnset:
				// Keep session root path empty.
			default:
				t.Fatalf("unexpected rootPath mode: %d", tt.rootPath)
			}

			flags := map[string]any{
				"-t": "demo:0",
				"-h": true,
			}
			switch tt.workDir {
			case workDirOmitted:
				// Simulates GUI split path where -c is not provided.
			case workDirEmpty:
				flags["-c"] = ""
			case workDirWhitespace:
				flags["-c"] = "   "
			case workDirValid:
				flags["-c"] = validDir
			default:
				t.Fatalf("unexpected workDir mode: %d", tt.workDir)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "split-window",
				Flags:   flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("split-window exit code = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.wantExitCode == 0 {
				paneID := strings.TrimSpace(resp.Stdout)
				if !strings.HasPrefix(paneID, "%") {
					t.Fatalf("split-window stdout = %q, want pane id", resp.Stdout)
				}
			} else if strings.TrimSpace(resp.Stderr) == "" {
				t.Fatal("split-window failure returned empty stderr")
			}

			if got := sessions.HasPane("%1"); got != tt.wantPaneOne {
				t.Fatalf("HasPane(%%1) = %v, want %v", got, tt.wantPaneOne)
			}
		})
	}
}

func TestSplitWindowWorkDirFallbackUsesWorktreePath(t *testing.T) {
	sessions := NewSessionManager()
	defer sessions.Close()

	router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})
	if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	missingRoot := filepath.Join(t.TempDir(), "missing-root")
	if err := sessions.SetRootPath("demo", missingRoot); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}

	worktreeDir := os.TempDir()
	if err := sessions.SetWorktreeInfo("demo", &SessionWorktreeInfo{Path: worktreeDir}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	resp := router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags: map[string]any{
			"-t": "demo:0",
			"-h": true,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("split-window exit code = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	paneID := strings.TrimSpace(resp.Stdout)
	if !strings.HasPrefix(paneID, "%") {
		t.Fatalf("split-window stdout = %q, want pane id", resp.Stdout)
	}
	if !sessions.HasPane("%1") {
		t.Fatal("HasPane(%1) = false, want true")
	}
}

func TestSplitWindowRollbackFailureHidesRollbackError(t *testing.T) {
	emitter := &captureEmitter{}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)

	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
	if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	attachErr := errors.New("attach failed")
	router.attachTerminalFn = func(pane *TmuxPane, _ string, _ map[string]string, _ *TmuxPane) error {
		if pane == nil {
			t.Fatal("pane is nil in attach hook")
		}
		if _, _, killErr := sessions.KillPane(pane.IDString()); killErr != nil {
			t.Fatalf("KillPane(%s) error = %v", pane.IDString(), killErr)
		}
		return attachErr
	}

	resp := router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags: map[string]any{
			"-t": "demo:0",
			"-h": true,
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
	if strings.Contains(resp.Stderr, "pane not found") {
		t.Fatalf("Stderr should not leak rollback pane kill error: %q", resp.Stderr)
	}
	if sessions.HasPane("%1") {
		t.Fatal("split pane %1 should not remain after rollback path")
	}
}

func TestHandleSelectPaneTitle(t *testing.T) {
	tests := []struct {
		name             string
		title            string
		setEmptyTitle    bool
		omitTarget       bool
		seedTitle        string
		wantExitCode     int
		wantTitle        string
		wantRenameEvent  bool
		wantFocusEvent   bool
		forceRenameError bool
	}{
		{
			name:            "set pane title with -T flag",
			title:           "boss1",
			wantExitCode:    0,
			wantTitle:       "boss1",
			wantRenameEvent: true,
			wantFocusEvent:  true,
		},
		{
			name:           "empty -T flag does not change title",
			title:          "",
			wantExitCode:   0,
			wantTitle:      "",
			wantFocusEvent: true,
		},
		{
			name:            "unicode pane title",
			title:           "ワーカー1",
			wantExitCode:    0,
			wantTitle:       "ワーカー1",
			wantRenameEvent: true,
			wantFocusEvent:  true,
		},
		{
			name:            "whitespace-padded title is trimmed",
			title:           "  padded  ",
			wantExitCode:    0,
			wantTitle:       "padded",
			wantRenameEvent: true,
			wantFocusEvent:  true,
		},
		{
			name:             "rename failure keeps command success and emits focus only",
			title:            "boss1",
			wantExitCode:     0,
			wantTitle:        "",
			wantRenameEvent:  false,
			wantFocusEvent:   true,
			forceRenameError: true,
		},
		{
			name:            "explicit empty -T clears pane title",
			setEmptyTitle:   true,
			seedTitle:       "seed",
			wantExitCode:    0,
			wantTitle:       "",
			wantRenameEvent: true,
			wantFocusEvent:  true,
		},
		{
			name:            "title-only command updates current pane without focus",
			title:           "solo",
			omitTarget:      true,
			wantExitCode:    0,
			wantTitle:       "solo",
			wantRenameEvent: true,
			wantFocusEvent:  false,
		},
		{
			name:            "title-only explicit empty -T clears current pane without focus",
			setEmptyTitle:   true,
			omitTarget:      true,
			seedTitle:       "seed",
			wantExitCode:    0,
			wantTitle:       "",
			wantRenameEvent: true,
			wantFocusEvent:  false,
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
			if tt.seedTitle != "" {
				if _, err := sessions.RenamePane("%0", tt.seedTitle); err != nil {
					t.Fatalf("RenamePane(seed) error = %v", err)
				}
			}

			if tt.forceRenameError {
				router.renamePane = func(string, string) (string, error) {
					return "", errors.New("injected rename failure")
				}
			}

			flags := map[string]any{}
			if !tt.omitTarget {
				flags["-t"] = "demo:0.0"
			}
			if tt.setEmptyTitle {
				flags["-T"] = ""
			} else if tt.title != "" {
				flags["-T"] = tt.title
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "select-pane",
				Flags:   flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			resolved, err := sessions.ResolveTarget("demo:0.0", -1)
			if err != nil {
				t.Fatalf("ResolveTarget() error = %v", err)
			}
			if resolved.Title != tt.wantTitle {
				t.Fatalf("pane title = %q, want %q", resolved.Title, tt.wantTitle)
			}

			events := emitter.Events()
			renameCount := 0
			focusCount := 0
			for _, ev := range events {
				switch ev.name {
				case "tmux:pane-renamed":
					renameCount++
					payload, ok := ev.payload.(map[string]any)
					if !ok {
						t.Fatalf("rename payload type = %T, want map[string]any", ev.payload)
					}
					if got := mustString(payload["sessionName"]); got != "demo" {
						t.Fatalf("rename payload sessionName = %q, want %q", got, "demo")
					}
					if got := mustString(payload["paneId"]); got != "%0" {
						t.Fatalf("rename payload paneId = %q, want %q", got, "%0")
					}
					if got := mustString(payload["title"]); got != tt.wantTitle {
						t.Fatalf("rename payload title = %q, want %q", got, tt.wantTitle)
					}
				case "tmux:pane-focused":
					focusCount++
					payload, ok := ev.payload.(map[string]any)
					if !ok {
						t.Fatalf("focus payload type = %T, want map[string]any", ev.payload)
					}
					if got := mustString(payload["sessionName"]); got != "demo" {
						t.Fatalf("focus payload sessionName = %q, want %q", got, "demo")
					}
					if got := mustString(payload["paneId"]); got != "%0" {
						t.Fatalf("focus payload paneId = %q, want %q", got, "%0")
					}
				}
			}

			if got := renameCount > 0; got != tt.wantRenameEvent {
				t.Fatalf("rename event emitted = %v, want %v (events: %v)", got, tt.wantRenameEvent, events)
			}
			if tt.wantRenameEvent && renameCount != 1 {
				t.Fatalf("rename event count = %d, want 1", renameCount)
			}

			if got := focusCount > 0; got != tt.wantFocusEvent {
				t.Fatalf("focus event emitted = %v, want %v (events: %v)", got, tt.wantFocusEvent, events)
			}
			if tt.wantFocusEvent && focusCount != 1 {
				t.Fatalf("focus event count = %d, want 1", focusCount)
			}
			if tt.wantRenameEvent && tt.wantFocusEvent {
				eventNames := emitter.EventNames()
				renameIdx := firstEventIndex(eventNames, "tmux:pane-renamed")
				focusIdx := firstEventIndex(eventNames, "tmux:pane-focused")
				if renameIdx < 0 || focusIdx < 0 {
					t.Fatalf("missing expected event order markers: events=%v", eventNames)
				}
				if renameIdx > focusIdx {
					t.Fatalf("event order mismatch: rename index=%d, focus index=%d, events=%v", renameIdx, focusIdx, eventNames)
				}
			}
		})
	}
}

func firstEventIndex(eventNames []string, want string) int {
	for i, name := range eventNames {
		if name == want {
			return i
		}
	}
	return -1
}

func TestHandleKillPane(t *testing.T) {
	tests := []struct {
		name             string
		splitBeforeKill  bool
		killTarget       string
		wantExitCode     int
		wantSessionGone  bool
		wantDestroyEvent bool
		wantLayoutEvent  bool
		wantErrSubstring string
	}{
		{
			name:             "kill only pane destroys session",
			splitBeforeKill:  false,
			killTarget:       "%0",
			wantExitCode:     0,
			wantSessionGone:  true,
			wantDestroyEvent: true,
			wantLayoutEvent:  false,
		},
		{
			name:             "kill one of two panes emits layout-changed",
			splitBeforeKill:  true,
			killTarget:       "%1",
			wantExitCode:     0,
			wantSessionGone:  false,
			wantDestroyEvent: false,
			wantLayoutEvent:  true,
		},
		{
			name:             "kill non-existent pane returns error",
			splitBeforeKill:  false,
			killTarget:       "%99",
			wantExitCode:     1,
			wantErrSubstring: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
			if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			if tt.splitBeforeKill {
				if _, err := sessions.SplitPane(0, SplitHorizontal); err != nil {
					t.Fatalf("SplitPane() error = %v", err)
				}
			}

			// Clear events from setup.
			emitter.mu.Lock()
			emitter.events = nil
			emitter.mu.Unlock()

			resp := router.Execute(ipc.TmuxRequest{
				Command: "kill-pane",
				Flags:   map[string]any{"-t": tt.killTarget},
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantErrSubstring != "" {
				if !strings.Contains(resp.Stderr, tt.wantErrSubstring) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantErrSubstring)
				}
				return
			}

			if tt.wantSessionGone {
				if sessions.HasSession("demo") {
					t.Fatalf("session 'demo' still exists, want destroyed")
				}
			} else {
				if !sessions.HasSession("demo") {
					t.Fatalf("session 'demo' not found, want still alive")
				}
			}

			events := emitter.Events()
			destroyCount := 0
			layoutCount := 0
			for _, ev := range events {
				switch ev.name {
				case "tmux:session-destroyed":
					destroyCount++
					payload, ok := ev.payload.(map[string]any)
					if !ok {
						t.Fatalf("session-destroyed payload type = %T, want map[string]any", ev.payload)
					}
					if got := mustString(payload["name"]); got != "demo" {
						t.Fatalf("session-destroyed name = %q, want %q", got, "demo")
					}
				case "tmux:layout-changed":
					layoutCount++
					payload, ok := ev.payload.(map[string]any)
					if !ok {
						t.Fatalf("layout-changed payload type = %T, want map[string]any", ev.payload)
					}
					if got := mustString(payload["sessionName"]); got != "demo" {
						t.Fatalf("layout-changed sessionName = %q, want %q", got, "demo")
					}
					if payload["layoutTree"] == nil {
						t.Fatalf("layout-changed layoutTree is nil")
					}
				}
			}

			if got := destroyCount > 0; got != tt.wantDestroyEvent {
				t.Fatalf("session-destroyed emitted = %v, want %v (events: %v)", got, tt.wantDestroyEvent, emitter.EventNames())
			}
			if tt.wantDestroyEvent && destroyCount != 1 {
				t.Fatalf("session-destroyed count = %d, want 1", destroyCount)
			}
			if got := layoutCount > 0; got != tt.wantLayoutEvent {
				t.Fatalf("layout-changed emitted = %v, want %v (events: %v)", got, tt.wantLayoutEvent, emitter.EventNames())
			}
			if tt.wantLayoutEvent && layoutCount != 1 {
				t.Fatalf("layout-changed count = %d, want 1", layoutCount)
			}
		})
	}
}

// TestHandleKillPaneTerminalClosedOnce verifies that kill-pane closes the
// terminal exactly once — via KillPane's internal closeTargets path.
// The redundant terminal.Close() in handleKillPane was removed as C-01.
// G-01: Mock Terminal で Close 呼び出し確認。
func TestHandleKillPaneTerminalClosedOnce(t *testing.T) {
	emitter := &captureEmitter{}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)

	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})
	if _, _, err := sessions.CreateSession("demo", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Inject a zero-value stub terminal to verify cleanup without ConPTY.
	// KillPane owns the terminal lifecycle (killPaneLocked -> closeTargets);
	// handleKillPane must not call Close() again.
	stub := &terminal.Terminal{}
	pane, err := sessions.ResolveTarget("%0", -1)
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	pane.Terminal = stub

	resp := router.Execute(ipc.TmuxRequest{
		Command: "kill-pane",
		Flags:   map[string]any{"-t": "%0"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	// Terminal must be closed by KillPane's internal path.
	if !stub.IsClosed() {
		t.Fatal("terminal.IsClosed() = false after kill-pane, want true (KillPane must close terminal)")
	}

	// Session destroyed because it had only one pane.
	if sessions.HasSession("demo") {
		t.Fatal("session 'demo' still exists, want destroyed after last pane killed")
	}
}

func TestHandleResizePane(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		flags            map[string]any
		wantExitCode     int
		wantWidth        int
		wantHeight       int
		wantErrSubstring string
	}{
		{
			name:         "resize both width and height",
			target:       "%0",
			flags:        map[string]any{"-x": 100, "-y": 30},
			wantExitCode: 0,
			wantWidth:    100,
			wantHeight:   30,
		},
		{
			name:         "resize only width keeps original height",
			target:       "%0",
			flags:        map[string]any{"-x": 80},
			wantExitCode: 0,
			wantWidth:    80,
			wantHeight:   40,
		},
		{
			name:         "resize only height keeps original width",
			target:       "%0",
			flags:        map[string]any{"-y": 25},
			wantExitCode: 0,
			wantWidth:    120,
			wantHeight:   25,
		},
		{
			name:             "non-existent pane returns error",
			target:           "%99",
			flags:            map[string]any{"-x": 100, "-y": 30},
			wantExitCode:     1,
			wantErrSubstring: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

			// Create session via router to attach a real terminal (ResizePane requires Terminal != nil).
			resp := router.Execute(ipc.TmuxRequest{
				Command: "new-session",
				Flags: map[string]any{
					"-s": "demo",
					"-x": 120,
					"-y": 40,
				},
			})
			if resp.ExitCode != 0 {
				t.Fatalf("new-session failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
			}
			emitter.mu.Lock()
			emitter.events = nil
			emitter.mu.Unlock()

			flags := make(map[string]any, len(tt.flags)+1)
			maps.Copy(flags, tt.flags)
			flags["-t"] = tt.target

			resp = router.Execute(ipc.TmuxRequest{
				Command: "resize-pane",
				Flags:   flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantErrSubstring != "" {
				if !strings.Contains(resp.Stderr, tt.wantErrSubstring) {
					t.Fatalf("Stderr = %q, want substring %q", resp.Stderr, tt.wantErrSubstring)
				}
				return
			}

			pane, err := sessions.ResolveTarget(tt.target, -1)
			if err != nil {
				t.Fatalf("ResolveTarget(%q) error = %v", tt.target, err)
			}
			if pane.Width != tt.wantWidth {
				t.Fatalf("Width = %d, want %d", pane.Width, tt.wantWidth)
			}
			if pane.Height != tt.wantHeight {
				t.Fatalf("Height = %d, want %d", pane.Height, tt.wantHeight)
			}

			layoutChangedCount := 0
			for _, ev := range emitter.Events() {
				if ev.name != "tmux:layout-changed" {
					continue
				}
				layoutChangedCount++
				payload, ok := ev.payload.(map[string]any)
				if !ok {
					t.Fatalf("layout-changed payload type = %T, want map[string]any", ev.payload)
				}
				if got := mustString(payload["sessionName"]); got != "demo" {
					t.Fatalf("layout-changed sessionName = %q, want %q", got, "demo")
				}
				if payload["layoutTree"] == nil {
					t.Fatal("layout-changed layoutTree is nil")
				}
			}
			if layoutChangedCount != 1 {
				t.Fatalf("layout-changed event count = %d, want 1 (events: %v)", layoutChangedCount, emitter.EventNames())
			}
		})
	}
}

func TestHandleSendKeysCopyMode(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantExitCode int
	}{
		{
			name:         "cancel command succeeds",
			args:         []string{"cancel"},
			wantExitCode: 0,
		},
		{
			name:         "Cancel case-insensitive succeeds",
			args:         []string{"Cancel"},
			wantExitCode: 0,
		},
		{
			name:         "known command page-up succeeds",
			args:         []string{"page-up"},
			wantExitCode: 0,
		},
		{
			name:         "unknown command silently succeeds",
			args:         []string{"select-word"},
			wantExitCode: 0,
		},
		{
			name:         "no args silently succeeds",
			args:         nil,
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)

			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

			// Create session via router to attach a real terminal.
			resp := router.Execute(ipc.TmuxRequest{
				Command: "new-session",
				Flags: map[string]any{
					"-s": "demo",
					"-x": 80,
					"-y": 24,
				},
			})
			if resp.ExitCode != 0 {
				t.Fatalf("new-session failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
			}

			resp = router.Execute(ipc.TmuxRequest{
				Command: "send-keys",
				Flags: map[string]any{
					"-t": "%0",
					"-X": true,
				},
				Args: tt.args,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
		})
	}
}

func TestHandleListPanesAllSessions(t *testing.T) {
	emitter := &captureEmitter{}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)

	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	// Create two sessions with panes.
	resp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-s": "alpha",
			"-x": 80,
			"-y": 24,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session alpha failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}

	resp = router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-s": "beta",
			"-x": 80,
			"-y": 24,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session beta failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}

	// list-panes -a should return panes from both sessions.
	resp = router.Execute(ipc.TmuxRequest{
		Command: "list-panes",
		Flags: map[string]any{
			"-a": true,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("list-panes -a failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}

	lines := strings.Split(strings.TrimRight(resp.Stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("list-panes -a returned %d lines, want at least 2 (one per session)", len(lines))
	}

	// With custom format, verify session_name is available.
	resp = router.Execute(ipc.TmuxRequest{
		Command: "list-panes",
		Flags: map[string]any{
			"-a": true,
			"-F": "#{session_name}:#{pane_id}",
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("list-panes -a -F failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}

	output := resp.Stdout
	if !strings.Contains(output, "alpha:") {
		t.Fatalf("list-panes -a -F output missing 'alpha:': %q", output)
	}
	if !strings.Contains(output, "beta:") {
		t.Fatalf("list-panes -a -F output missing 'beta:': %q", output)
	}
}

func TestHandleSendKeysMousePassthrough(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	session, _, err := sessions.CreateSession("test", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	pane := session.Windows[0].Panes[0]
	pane.Terminal = &terminal.Terminal{}

	resp := router.Execute(ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": pane.IDString(),
			"-M": true,
		},
		Args: []string{"some-mouse-event"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("send-keys -M exit code = %d, want 0, stderr = %q", resp.ExitCode, resp.Stderr)
	}
	if resp.Stdout != "" {
		t.Fatalf("send-keys -M stdout = %q, want empty", resp.Stdout)
	}
}

// TestHandleSendKeysMousePassthroughNilTerminal verifies that -M succeeds even
// when the target pane has no Terminal (the whole point of C-1 fix).
func TestHandleSendKeysMousePassthroughNilTerminal(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	session, _, err := sessions.CreateSession("test", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	pane := session.Windows[0].Panes[0]
	// Explicitly leave Terminal as nil to test the C-1 fix.
	pane.Terminal = nil

	resp := router.Execute(ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": pane.IDString(),
			"-M": true,
		},
		Args: []string{"mouse-data"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("send-keys -M with nil Terminal: exit code = %d, want 0, stderr = %q",
			resp.ExitCode, resp.Stderr)
	}
}

func TestHandleCopyMode(t *testing.T) {
	tests := []struct {
		name      string
		quit      bool
		wantEvent string
	}{
		{
			name:      "enter copy mode",
			quit:      false,
			wantEvent: "tmux:copy-mode-enter",
		},
		{
			name:      "exit copy mode",
			quit:      true,
			wantEvent: "tmux:copy-mode-exit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			emitter := &captureEmitter{}
			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

			session, _, err := sessions.CreateSession("test", "0", 120, 40)
			if err != nil {
				t.Fatalf("CreateSession error: %v", err)
			}
			pane := session.Windows[0].Panes[0]

			flags := map[string]any{"-t": pane.IDString()}
			if tt.quit {
				flags["-q"] = true
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "copy-mode",
				Flags:   flags,
			})
			if resp.ExitCode != 0 {
				t.Fatalf("copy-mode exit code = %d, want 0, stderr = %q", resp.ExitCode, resp.Stderr)
			}

			events := emitter.Events()
			found := false
			for _, ev := range events {
				if ev.name == tt.wantEvent {
					payload := ev.payload.(map[string]any)
					if payload["paneId"] != pane.IDString() {
						t.Fatalf("event paneId = %q, want %q", payload["paneId"], pane.IDString())
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected event %q not found in %v", tt.wantEvent, emitter.EventNames())
			}
		})
	}
}

func TestHandleCopyModeInvalidTarget(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, nil, RouterOptions{})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "copy-mode",
		Flags:   map[string]any{"-t": "%999"},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("copy-mode with invalid target exit code = %d, want 1", resp.ExitCode)
	}
}

func TestHandleListPanesWithFilter(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	// Create session with pane.
	if _, _, err := sessions.CreateSession("test", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}

	// Filter that matches active pane (pane_active = "1").
	resp := router.Execute(ipc.TmuxRequest{
		Command:    "list-panes",
		Flags:      map[string]any{"-a": true, "-f": "#{pane_active}"},
		CallerPane: "%0",
	})
	if resp.ExitCode != 0 {
		t.Fatalf("list-panes -f exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
	}
	if strings.TrimSpace(resp.Stdout) == "" {
		t.Fatal("list-panes with active filter should return at least one pane")
	}

	// Filter that matches nothing (equality check).
	resp = router.Execute(ipc.TmuxRequest{
		Command:    "list-panes",
		Flags:      map[string]any{"-a": true, "-f": "#{==:#{session_name},nonexistent}"},
		CallerPane: "%0",
	})
	if resp.ExitCode != 0 {
		t.Fatalf("list-panes -f exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
	}
	if strings.TrimSpace(resp.Stdout) != "" {
		t.Fatalf("list-panes with nonexistent filter should return empty, got %q", resp.Stdout)
	}
}

func TestHandleListPanesFilterWithoutAllFlag(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	// Create session via router for proper terminal.
	resp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{"-s": "demo", "-x": 120, "-y": 40},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}
	// Split to have two panes.
	resp = router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{"-t": "demo:0", "-h": true},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("split-window failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}

	tests := []struct {
		name      string
		filter    string
		format    string
		wantLines int
	}{
		{
			name:      "active pane filter",
			filter:    "#{pane_active}",
			format:    "#{pane_id}",
			wantLines: 1,
		},
		{
			name:      "zero filter excludes all",
			filter:    "0",
			format:    "#{pane_id}",
			wantLines: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := router.Execute(ipc.TmuxRequest{
				Command:    "list-panes",
				Flags:      map[string]any{"-t": "demo", "-f": tt.filter, "-F": tt.format},
				CallerPane: "%0",
			})
			if resp.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
			}
			output := strings.TrimRight(resp.Stdout, "\n")
			var lines []string
			if output != "" {
				lines = strings.Split(output, "\n")
			}
			if len(lines) != tt.wantLines {
				t.Fatalf("lines = %d, want %d, stdout = %q", len(lines), tt.wantLines, resp.Stdout)
			}
		})
	}
}

func TestHandleCopyModeEventPayload(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	session, _, err := sessions.CreateSession("test", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	pane := session.Windows[0].Panes[0]

	resp := router.Execute(ipc.TmuxRequest{
		Command: "copy-mode",
		Flags:   map[string]any{"-t": pane.IDString()},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("copy-mode exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
	}

	events := emitter.Events()
	found := false
	for _, ev := range events {
		if ev.name == "tmux:copy-mode-enter" {
			payload := ev.payload.(map[string]any)
			sessionName, ok := payload["sessionName"].(string)
			if !ok {
				t.Fatal("sessionName missing from event payload")
			}
			if sessionName != "test" {
				t.Fatalf("sessionName = %q, want %q", sessionName, "test")
			}
			paneId, ok := payload["paneId"].(string)
			if !ok {
				t.Fatal("paneId missing from event payload")
			}
			if paneId != pane.IDString() {
				t.Fatalf("paneId = %q, want %q", paneId, pane.IDString())
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("tmux:copy-mode-enter event not found")
	}
}

func TestHandleListPanesFilterVerifiesPaneIDs(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	// Create two sessions.
	resp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{"-s": "alpha", "-x": 80, "-y": 24},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session alpha failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}
	resp = router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{"-s": "beta", "-x": 80, "-y": 24},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session beta failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
	}

	// list-panes -a -f filtering by session name should include matching pane IDs only.
	resp = router.Execute(ipc.TmuxRequest{
		Command: "list-panes",
		Flags:   map[string]any{"-a": true, "-f": "#{==:#{session_name},alpha}", "-F": "#{session_name}:#{pane_id}"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("list-panes exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
	}
	if !strings.Contains(resp.Stdout, "alpha:") {
		t.Fatalf("stdout should contain 'alpha:' but got %q", resp.Stdout)
	}
	if strings.Contains(resp.Stdout, "beta:") {
		t.Fatalf("stdout should NOT contain 'beta:' but got %q", resp.Stdout)
	}
}

func TestHandleListPanesAllSessionsEmpty(t *testing.T) {
	emitter := &captureEmitter{}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)

	router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

	// list-panes -a with no sessions should return empty.
	resp := router.Execute(ipc.TmuxRequest{
		Command: "list-panes",
		Flags: map[string]any{
			"-a": true,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	if strings.TrimSpace(resp.Stdout) != "" {
		t.Fatalf("Stdout = %q, want empty", resp.Stdout)
	}
}
