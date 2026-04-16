package main

import (
	"testing"

	"myT-x/internal/singletaskrunner"
)

func TestSingleTaskRunnerMCPDefinitions(t *testing.T) {
	defs := singleTaskRunnerMCPDefinitions()
	if len(defs) != 1 {
		t.Fatalf("definitions count = %d, want 1", len(defs))
	}

	def := defs[0]
	if def.ID != "single-task-runner" {
		t.Fatalf("ID = %q, want %q", def.ID, "single-task-runner")
	}
	if def.Kind != "single-task-runner" {
		t.Fatalf("Kind = %q, want %q", def.Kind, "single-task-runner")
	}
	if def.DefaultEnabled {
		t.Fatal("DefaultEnabled = true, want false")
	}
	wantDescription := "Sequentially dispatches queued tasks to their configured target panes and waits for explicit " + singletaskrunner.ResolutionToolNames + " MCP tool calls."
	if def.Description != wantDescription {
		t.Fatalf("Description = %q", def.Description)
	}
}
