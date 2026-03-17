package tmux

import (
	"reflect"
	"testing"
)

func TestRouterOptionsStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[RouterOptions]().NumField(); got != 10 {
		t.Fatalf("RouterOptions field count = %d, want 10 (DefaultShell, PipeName, HostPID, ShimAvailable, PaneEnv, ClaudeEnv, OnSessionDestroyed, OnSessionRenamed, ResolveMCPStdio, ResolveSessionByCwd)", got)
	}
}
