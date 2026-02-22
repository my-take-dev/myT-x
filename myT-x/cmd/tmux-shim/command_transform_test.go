package main

import (
	"reflect"
	"runtime"
	"slices"
	"testing"

	"myT-x/internal/ipc"
)

func TestApplyShellTransformNewSession(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-c": `C:\existing`,
		},
		Env: map[string]string{
			"EXISTING":   "1",
			"CLAUDECODE": "0",
		},
		Args: []string{
			`cd 'C:\workspace' && CLAUDECODE=1 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 'C:\Users\test\.local\bin\claude.exe' --agent-name reviewer`,
		},
	}
	before := cloneTransformRequest(&req)

	changed := applyShellTransform(&req)
	if runtime.GOOS != "windows" {
		if changed {
			t.Fatal("non-windows request should not be transformed")
		}
		if !reflect.DeepEqual(req, before) {
			t.Fatalf("non-windows request mutated: got %#v, want %#v", req, before)
		}
		return
	}
	if !changed {
		t.Fatal("expected shell transform to change request on windows")
	}
	if got := asString(req.Flags["-c"]); got != `C:\workspace` {
		t.Fatalf("-c = %q, want %q", got, `C:\workspace`)
	}
	if got := req.Env["CLAUDECODE"]; got != "1" {
		t.Fatalf("CLAUDECODE = %q, want 1", got)
	}
	if got := req.Env["CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"]; got != "1" {
		t.Fatalf("CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS = %q, want 1", got)
	}
	if len(req.Args) != 1 {
		t.Fatalf("args length = %d, want 1", len(req.Args))
	}
	want := `& 'C:\Users\test\.local\bin\claude.exe' --agent-name reviewer`
	if req.Args[0] != want {
		t.Fatalf("args[0] = %q, want %q", req.Args[0], want)
	}
}

func TestApplyShellTransformSplitWindowFallback(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{},
		Env:     map[string]string{},
		Args:    []string{"echo foo && echo bar"},
	}
	before := cloneTransformRequest(&req)

	changed := applyShellTransform(&req)
	if runtime.GOOS != "windows" {
		if changed {
			t.Fatal("non-windows split-window should not be transformed")
		}
		if !reflect.DeepEqual(req, before) {
			t.Fatalf("non-windows request mutated: got %#v, want %#v", req, before)
		}
		return
	}
	if !changed {
		t.Fatal("expected shell transform to replace && on windows")
	}
	if got := req.Args[0]; got != "echo foo ; echo bar" {
		t.Fatalf("args[0] = %q, want %q", got, "echo foo ; echo bar")
	}
}

func TestApplyShellTransformSendKeys(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "send-keys",
		Flags:   map[string]any{"-t": "%1"},
		Env:     map[string]string{},
		Args: []string{
			`cd 'C:\workspace' && CLAUDECODE=1 'C:\bin\claude.exe' --resume abc`,
			"Enter",
		},
	}
	before := cloneTransformRequest(&req)

	changed := applyShellTransform(&req)
	if runtime.GOOS != "windows" {
		if changed {
			t.Fatal("non-windows send-keys should not be transformed")
		}
		if !reflect.DeepEqual(req, before) {
			t.Fatalf("non-windows request mutated: got %#v, want %#v", req, before)
		}
		return
	}
	if !changed {
		t.Fatal("expected send-keys transform to change args on windows")
	}
	want := `cd 'C:\workspace'; $env:CLAUDECODE='1'; & 'C:\bin\claude.exe' --resume abc`
	if got := req.Args[0]; got != want {
		t.Fatalf("args[0] = %q, want %q", got, want)
	}
	if req.Args[1] != "Enter" {
		t.Fatalf("args[1] = %q, want Enter", req.Args[1])
	}
}

func TestApplyShellTransformNonTargetCommand(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "list-sessions",
		Flags:   map[string]any{},
		Env:     map[string]string{},
		Args:    []string{"echo foo && echo bar"},
	}

	changed := applyShellTransform(&req)
	if changed {
		t.Fatal("non-target command should not be transformed")
	}
	if req.Args[0] != "echo foo && echo bar" {
		t.Fatalf("args unexpectedly changed: %q", req.Args[0])
	}
}

func TestApplyShellTransformNilRequest(t *testing.T) {
	if changed := applyShellTransform(nil); changed {
		t.Fatal("nil request should not be transformed")
	}
}

func TestApplyShellTransformInitializesNilMaps(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Flags:   nil,
		Env:     nil,
		Args:    []string{"echo foo"},
	}

	changed := applyShellTransform(&req)
	if changed {
		t.Fatal("split-window without shell patterns should be unchanged")
	}
	if req.Flags == nil {
		t.Fatal("Flags should be initialized")
	}
	if req.Env == nil {
		t.Fatal("Env should be initialized")
	}
}

// --- I-31: Direct unit tests for transform functions ---

func TestApplyNewProcessTransformDirectCases(t *testing.T) {
	tests := []struct {
		name        string
		req         ipc.TmuxRequest
		wantChanged bool
	}{
		{
			name: "nil flags and env are handled",
			req: ipc.TmuxRequest{
				Command: "new-session",
				Flags:   nil,
				Env:     nil,
				Args:    []string{"echo hello"},
			},
		},
		{
			name: "empty args no change",
			req: ipc.TmuxRequest{
				Command: "new-window",
				Flags:   map[string]any{},
				Env:     map[string]string{},
				Args:    []string{},
			},
		},
		{
			name: "nil args no change",
			req: ipc.TmuxRequest{
				Command: "split-window",
				Flags:   map[string]any{},
				Env:     map[string]string{},
				Args:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// applyNewProcessTransform should not panic on edge cases
			changed := applyNewProcessTransform(&tt.req)
			if tt.wantChanged != changed {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

func TestApplySendKeysTransformDirectCases(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantChanged bool
	}{
		{
			name:        "empty args no change",
			args:        []string{},
			wantChanged: false,
		},
		{
			name:        "nil args no change",
			args:        nil,
			wantChanged: false,
		},
		{
			name:        "plain text no change",
			args:        []string{"echo hello", "Enter"},
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Flags:   map[string]any{"-t": "%0"},
				Env:     map[string]string{},
				Args:    tt.args,
			}
			changed := applySendKeysTransform(&req)
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

func TestApplyShellTransformNewWindowSameAsNewSession(t *testing.T) {
	// Verify new-window follows the same transform path as new-session/split-window
	req := ipc.TmuxRequest{
		Command: "new-window",
		Flags:   map[string]any{},
		Env:     map[string]string{},
		Args:    []string{"echo test"},
	}

	changed := applyShellTransform(&req)
	// On non-windows, no change; on windows, depends on content
	if runtime.GOOS != "windows" {
		if changed {
			t.Fatal("non-windows new-window should not be transformed")
		}
	}
}

// TestApplyNewProcessTransformWindowsPathAndEnv is a direct test of Windows path and environment variable transformation.
func TestApplyNewProcessTransformWindowsPathAndEnv(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only transform test")
	}
	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{"-c": `C:\start`},
		Env:     map[string]string{},
		Args: []string{
			`cd 'C:\projects\foo' && BAR=1 'C:\bin\app.exe' --flag`,
		},
	}
	changed := applyNewProcessTransform(&req)
	if !changed {
		t.Fatal("expected change on windows")
	}
	if got := asString(req.Flags["-c"]); got != `C:\projects\foo` {
		t.Fatalf("-c = %q, want %q", got, `C:\projects\foo`)
	}
	if req.Env["BAR"] != "1" {
		t.Fatalf("BAR env = %q, want 1", req.Env["BAR"])
	}
	// Args should be cleaned up (cd removed, env removed)
	if len(req.Args) == 0 {
		t.Fatal("Args should not be empty after transform")
	}
}

// TestApplySendKeysTransformWithWindowsArgs is a direct test of Windows command transformation in send-keys.
func TestApplySendKeysTransformWithWindowsArgs(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only send-keys transform test")
	}
	req := ipc.TmuxRequest{
		Command: "send-keys",
		Flags:   map[string]any{"-t": "%1"},
		Env:     map[string]string{},
		Args: []string{
			`cd 'C:\projects' && FOO=bar 'C:\bin\claude.exe' --resume abc`,
			"Enter",
		},
	}
	changed := applySendKeysTransform(&req)
	if !changed {
		t.Fatal("expected change on windows with unix-style command")
	}
	// After translation, Enter should still be present
	hasEnter := slices.Contains(req.Args, "Enter")
	if !hasEnter {
		t.Fatalf("args after transform = %v, expected to contain Enter", req.Args)
	}
}

// TestApplyNewProcessTransformNoChangeWhenCleanArgs verifies idempotence when args
// contain no shell expansion patterns (no cd, no KEY=VAL, no quoted executable).
// Uses a pre-joined single-arg form to avoid platform-specific join normalisation
// on Windows (ParseUnixCommand always joins []string into one string).
func TestApplyNewProcessTransformNoChangeWhenCleanArgs(t *testing.T) {
	// Single-arg form is idempotent on all platforms:
	// - non-Windows: ParseUnixCommand returns CleanArgs == input slice
	// - Windows:     join("powershell.exe -Command Get-Process") == same string -> no change
	req := ipc.TmuxRequest{
		Command: "new-session",
		Flags:   map[string]any{},
		Env:     map[string]string{},
		Args:    []string{"powershell.exe -Command Get-Process"},
	}
	origArg := req.Args[0]
	changed := applyNewProcessTransform(&req)
	if changed {
		t.Fatalf("changed = true, want false for clean single-arg input")
	}
	if len(req.Args) != 1 || req.Args[0] != origArg {
		t.Fatalf("args changed unexpectedly: got %v, want [%q]", req.Args, origArg)
	}
}
