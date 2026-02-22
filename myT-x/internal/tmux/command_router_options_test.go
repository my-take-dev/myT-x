package tmux

import (
	"reflect"
	"testing"
)

func TestRouterOptionsStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[RouterOptions]().NumField(); got != 6 {
		t.Fatalf("RouterOptions field count = %d, want 6 (DefaultShell, PipeName, HostPID, ShimAvailable, PaneEnv, ClaudeEnv)", got)
	}
}
