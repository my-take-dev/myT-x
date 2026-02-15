package tmux

import (
	"reflect"
	"testing"
)

func TestRouterOptionsStructFieldCounts(t *testing.T) {
	if got := reflect.TypeOf(RouterOptions{}).NumField(); got != 5 {
		t.Fatalf("RouterOptions field count = %d, want 5 (DefaultShell, PipeName, HostPID, ShimAvailable, PaneEnv)", got)
	}
}
