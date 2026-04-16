package mcp

import "testing"

func TestDefinitionKindIsBuiltIn(t *testing.T) {
	tests := []struct {
		name string
		kind DefinitionKind
		want bool
	}{
		{name: "default lsp", kind: DefinitionKindLSP, want: true},
		{name: "orchestrator", kind: DefinitionKindOrchestrator, want: true},
		{name: "single task runner", kind: DefinitionKindSingleTaskRunner, want: true},
		{name: "custom command kind", kind: DefinitionKindCustom, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.IsBuiltIn(); got != tt.want {
				t.Fatalf("IsBuiltIn(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestDefinitionKindUsesEmbeddedRuntime(t *testing.T) {
	tests := []struct {
		name string
		kind DefinitionKind
		want bool
	}{
		{name: "default lsp", kind: DefinitionKindLSP, want: false},
		{name: "orchestrator", kind: DefinitionKindOrchestrator, want: true},
		{name: "single task runner", kind: DefinitionKindSingleTaskRunner, want: true},
		{name: "custom command kind", kind: DefinitionKindCustom, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.UsesEmbeddedRuntime(); got != tt.want {
				t.Fatalf("UsesEmbeddedRuntime(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}
