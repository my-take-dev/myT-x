package main

import "testing"

func TestOrchestratorMCPDefinitions_HasSessionAllPanesConfigParam(t *testing.T) {
	defs := orchestratorMCPDefinitions()
	if len(defs) != 1 {
		t.Fatalf("definitions count = %d, want 1", len(defs))
	}
	def := defs[0]
	if def.Kind != "orchestrator" {
		t.Fatalf("Kind = %q, want %q", def.Kind, "orchestrator")
	}
	found := false
	for _, p := range def.ConfigParams {
		if p.Key == "session_all_panes" {
			found = true
			if p.DefaultValue != "false" {
				t.Fatalf("session_all_panes DefaultValue = %q, want %q", p.DefaultValue, "false")
			}
		}
	}
	if !found {
		t.Fatal("session_all_panes ConfigParam not found in orchestrator definition")
	}
}
