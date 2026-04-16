package taskscheduler

import (
	"slices"
	"testing"

	"myT-x/internal/config"
)

func TestPreExecTargetModesMatchConfigModes(t *testing.T) {
	got := []PreExecTargetMode{
		PreExecTargetModeTaskPanes,
		PreExecTargetModeAllPanes,
	}

	if !slices.Equal(got, config.AllTaskSchedulerPreExecTargetModes()) {
		t.Fatalf("PreExecTargetMode set = %v, want %v", got, config.AllTaskSchedulerPreExecTargetModes())
	}

	for _, mode := range got {
		if !config.IsValidTaskSchedulerPreExecTargetMode(mode) {
			t.Fatalf("PreExecTargetMode %q is not accepted by config validation", mode)
		}
	}
}
