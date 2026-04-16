package tmux

import (
	"reflect"
	"testing"
)

func TestRouterOptionsStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[RouterOptions]().NumField(); got != 11 {
		t.Fatalf("RouterOptions field count = %d, want 11 (DefaultShell, PipeName, HostPID, ShimAvailable, PaneEnv, ClaudeEnv, OnSessionDestroyed, OnSessionRenamed, OnSessionRenameRollbackFailed, ResolveMCPStdio, ResolveSessionByCwd)", got)
	}
}
