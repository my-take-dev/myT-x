package tmux

import (
	"reflect"
	"testing"
)

func TestRouterOptionsStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[RouterOptions]().NumField(); got != 9 {
		t.Fatalf("RouterOptions field count = %d, want 9 (DefaultShell, PipeName, HostPID, ShimAvailable, PaneEnv, ClaudeEnv, OnSessionDestroyed, OnSessionRenamed, ResolveMCPStdio)", got)
	}
}
