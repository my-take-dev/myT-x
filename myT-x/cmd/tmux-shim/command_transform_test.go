package main

import (
	"reflect"
	"runtime"
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
