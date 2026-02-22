package ipc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultPipeNameHonorsTrustedEnvOverride(t *testing.T) {
	t.Setenv("GO_TMUX_PIPE", `\\.\pipe\myT-x-ci_pipe`)

	if got := DefaultPipeName(); got != `\\.\pipe\myT-x-ci_pipe` {
		t.Fatalf("DefaultPipeName() = %q, want trusted env override", got)
	}
}

func TestDefaultPipeNameRejectsUntrustedEnvOverride(t *testing.T) {
	t.Setenv("GO_TMUX_PIPE", `\\.\pipe\other-app`)
	t.Setenv("USERNAME", "unit-tester")

	got := DefaultPipeName()
	if got == `\\.\pipe\other-app` {
		t.Fatalf("DefaultPipeName() unexpectedly accepted untrusted env override")
	}
	if !strings.HasPrefix(got, defaultPipePrefix) {
		t.Fatalf("DefaultPipeName() = %q, want %q prefix", got, defaultPipePrefix)
	}
}

func TestDefaultPipeNameSanitizesUsername(t *testing.T) {
	t.Setenv("GO_TMUX_PIPE", "")
	t.Setenv("USERNAME", "unit user!")

	got := DefaultPipeName()
	want := `\\.\pipe\myT-x-unit_user_`
	if got != want {
		t.Fatalf("DefaultPipeName() = %q, want %q", got, want)
	}
}

func TestDefaultPipeNameFallbackWhenUsernameEmpty(t *testing.T) {
	t.Setenv("GO_TMUX_PIPE", "")
	t.Setenv("USERNAME", "")

	got := DefaultPipeName()

	// When USERNAME is empty, user.Current() may succeed (returning OS user)
	// or fail (returning "unknown" via sanitizeUsername fallback).
	// Either way the pipe name must have a non-empty suffix after the prefix.
	if !strings.HasPrefix(got, defaultPipePrefix) {
		t.Fatalf("DefaultPipeName() = %q, want prefix %q", got, defaultPipePrefix)
	}
	suffix := strings.TrimPrefix(got, defaultPipePrefix)
	if suffix == "" {
		t.Fatalf("DefaultPipeName() = %q, suffix after prefix must not be empty", got)
	}
}

func TestDecodeRequest_NilFieldsInitializedToEmpty(t *testing.T) {
	// JSON with only "command" — Flags, Args, Env are absent (will be nil after unmarshal).
	raw, err := json.Marshal(map[string]any{"command": "list-sessions"})
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	req, err := decodeRequest(raw)
	if err != nil {
		t.Fatalf("decodeRequest error = %v", err)
	}

	if req.Flags == nil {
		t.Error("decodeRequest: Flags is nil, want empty map")
	}
	if req.Args == nil {
		t.Error("decodeRequest: Args is nil, want empty slice")
	}
	if req.Env == nil {
		t.Error("decodeRequest: Env is nil, want empty map")
	}

	// Verify they are truly empty, not just non-nil.
	if len(req.Flags) != 0 {
		t.Errorf("decodeRequest: Flags has %d entries, want 0", len(req.Flags))
	}
	if len(req.Args) != 0 {
		t.Errorf("decodeRequest: Args has %d entries, want 0", len(req.Args))
	}
	if len(req.Env) != 0 {
		t.Errorf("decodeRequest: Env has %d entries, want 0", len(req.Env))
	}
}

func TestDecodeRequest_PreservesExplicitValues(t *testing.T) {
	input := TmuxRequest{
		Command: "send-keys",
		Flags:   map[string]any{"t": "main:0.0"},
		Args:    []string{"ls", "-la"},
		Env:     map[string]string{"TERM": "xterm"},
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	req, err := decodeRequest(raw)
	if err != nil {
		t.Fatalf("decodeRequest error = %v", err)
	}

	if len(req.Args) != 2 || req.Args[0] != "ls" || req.Args[1] != "-la" {
		t.Errorf("decodeRequest: Args = %v, want [ls -la]", req.Args)
	}
	if len(req.Flags) != 1 {
		t.Errorf("decodeRequest: Flags = %v, want 1 entry", req.Flags)
	}
	if len(req.Env) != 1 {
		t.Errorf("decodeRequest: Env = %v, want 1 entry", req.Env)
	}
}
