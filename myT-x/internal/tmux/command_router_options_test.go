package tmux

import (
	"reflect"
	"testing"
)

func TestRouterOptionsStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[RouterOptions]().NumField(); got != 8 {
		t.Fatalf("RouterOptions field count = %d, want 8 (DefaultShell, PipeName, HostPID, ShimAvailable, PaneEnv, ClaudeEnv, OnSessionDestroyed, OnSessionRenamed)", got)
	}
}
