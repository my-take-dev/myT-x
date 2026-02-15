package ipc

import (
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
