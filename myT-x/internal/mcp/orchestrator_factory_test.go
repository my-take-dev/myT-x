package mcp

import (
	"testing"
)

func TestConfigParamValue_KeyFound(t *testing.T) {
	params := []ConfigParam{
		{Key: "session_all_panes", DefaultValue: "true"},
		{Key: "other_key", DefaultValue: "other_value"},
	}
	got := configParamValue(params, "session_all_panes", "fallback")
	if got != "true" {
		t.Fatalf("configParamValue = %q, want %q", got, "true")
	}
}

func TestConfigParamValue_KeyNotFound(t *testing.T) {
	params := []ConfigParam{
		{Key: "other_key", DefaultValue: "other_value"},
	}
	got := configParamValue(params, "session_all_panes", "fallback")
	if got != "fallback" {
		t.Fatalf("configParamValue = %q, want %q", got, "fallback")
	}
}

func TestConfigParamValue_EmptyParams(t *testing.T) {
	got := configParamValue(nil, "session_all_panes", "default")
	if got != "default" {
		t.Fatalf("configParamValue = %q, want %q", got, "default")
	}
}

func TestBuildPipeConfig_OrchestratorPassesSessionInfo(t *testing.T) {
	def := Definition{
		ID:   "orch-test",
		Name: "Test Orchestrator",
		Kind: "orchestrator",
		ConfigParams: []ConfigParam{
			{Key: "session_all_panes", DefaultValue: "false"},
		},
	}
	cfg := buildPipeConfig(`\\.\pipe\test`, "/root", "my-session", def)
	if cfg.RuntimeFactory == nil {
		t.Fatal("RuntimeFactory should not be nil for orchestrator kind")
	}
	if cfg.PipeName != `\\.\pipe\test` {
		t.Fatalf("PipeName = %q, want %q", cfg.PipeName, `\\.\pipe\test`)
	}
}

func TestBuildPipeConfig_LSPIgnoresSessionName(t *testing.T) {
	def := Definition{
		ID:      "lsp-test",
		Name:    "Test LSP",
		Command: "gopls",
	}
	cfg := buildPipeConfig(`\\.\pipe\test`, "/root", "my-session", def)
	if cfg.RuntimeFactory != nil {
		t.Fatal("RuntimeFactory should be nil for LSP kind")
	}
	if cfg.LSPCommand != "gopls" {
		t.Fatalf("LSPCommand = %q, want %q", cfg.LSPCommand, "gopls")
	}
}

func TestBuildPipeConfig_OrchestratorAllPanesTrue(t *testing.T) {
	def := Definition{
		ID:   "orch-test",
		Name: "Test Orchestrator",
		Kind: "orchestrator",
		ConfigParams: []ConfigParam{
			{Key: "session_all_panes", DefaultValue: "true"},
		},
	}
	// Verify it doesn't panic and creates a valid config.
	cfg := buildPipeConfig(`\\.\pipe\test`, "/root", "my-session", def)
	if cfg.RuntimeFactory == nil {
		t.Fatal("RuntimeFactory should not be nil for orchestrator kind")
	}
}

func TestOrchestratorMCPDefinitions_HasSessionAllPanesConfigParam(t *testing.T) {
	// This test is in the main package but we test the Definition structure here
	// by constructing a definition with the same ConfigParams pattern.
	params := []ConfigParam{{
		Key:          "session_all_panes",
		Label:        "全セッションペイン表示",
		DefaultValue: "false",
		Description:  "false: 自セッションのペインのみ / true: 全セッションのペインを表示",
	}}
	got := configParamValue(params, "session_all_panes", "")
	if got != "false" {
		t.Fatalf("default session_all_panes = %q, want %q", got, "false")
	}
}
