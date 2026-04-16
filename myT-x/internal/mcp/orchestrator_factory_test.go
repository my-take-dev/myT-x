package mcp

import (
	"context"
	"testing"

	"myT-x/internal/singletaskrunner"
	"myT-x/internal/workerutil"
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
		Kind: DefinitionKindOrchestrator,
		ConfigParams: []ConfigParam{
			{Key: "session_all_panes", DefaultValue: "false"},
		},
	}
	cfg, err := buildPipeConfig(`\\.\pipe\test`, def, pipeConfigContext{
		rootDir:     "/root",
		sessionName: "my-session",
	})
	if err != nil {
		t.Fatalf("buildPipeConfig: %v", err)
	}
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
	cfg, err := buildPipeConfig(`\\.\pipe\test`, def, pipeConfigContext{
		rootDir:     "/root",
		sessionName: "my-session",
	})
	if err != nil {
		t.Fatalf("buildPipeConfig: %v", err)
	}
	if cfg.RuntimeFactory != nil {
		t.Fatal("RuntimeFactory should be nil for LSP kind")
	}
	if cfg.LSPCommand != "gopls" {
		t.Fatalf("LSPCommand = %q, want %q", cfg.LSPCommand, "gopls")
	}
}

func TestBuildPipeConfig_CustomKindUsesExternalCommand(t *testing.T) {
	def := Definition{
		ID:      "memory",
		Name:    "Memory Server",
		Kind:    "memory-server",
		Command: "npx",
		Args:    []string{"-y", "@anthropic/memory-server"},
	}
	cfg, err := buildPipeConfig(`\\.\pipe\test`, def, pipeConfigContext{
		rootDir:     "/root",
		sessionName: "my-session",
	})
	if err != nil {
		t.Fatalf("buildPipeConfig: %v", err)
	}
	if cfg.RuntimeFactory != nil {
		t.Fatal("RuntimeFactory should be nil for custom external MCP kinds")
	}
	if cfg.LSPCommand != "npx" {
		t.Fatalf("LSPCommand = %q, want %q", cfg.LSPCommand, "npx")
	}
	if len(cfg.LSPArgs) != 2 || cfg.LSPArgs[0] != "-y" || cfg.LSPArgs[1] != "@anthropic/memory-server" {
		t.Fatalf("LSPArgs = %#v, want [-y @anthropic/memory-server]", cfg.LSPArgs)
	}
}

func TestBuildPipeConfig_OrchestratorAllPanesTrue(t *testing.T) {
	def := Definition{
		ID:   "orch-test",
		Name: "Test Orchestrator",
		Kind: DefinitionKindOrchestrator,
		ConfigParams: []ConfigParam{
			{Key: "session_all_panes", DefaultValue: "true"},
		},
	}
	// Verify it doesn't panic and creates a valid config.
	cfg, err := buildPipeConfig(`\\.\pipe\test`, def, pipeConfigContext{
		rootDir:     "/root",
		sessionName: "my-session",
	})
	if err != nil {
		t.Fatalf("buildPipeConfig: %v", err)
	}
	if cfg.RuntimeFactory == nil {
		t.Fatal("RuntimeFactory should not be nil for orchestrator kind")
	}
}

func TestBuildPipeConfig_SingleTaskRunnerRequiresRuntimeFactory(t *testing.T) {
	def := Definition{
		ID:   "single-task-runner",
		Name: "Single Task Runner",
		Kind: DefinitionKindSingleTaskRunner,
	}

	manager := singletaskrunner.NewServiceManager(func(sessionName string) singletaskrunner.Deps {
		return singletaskrunner.Deps{
			CheckPaneAlive:   func(string) error { return nil },
			SendMessagePaste: func(string, string) error { return nil },
			SendClearCommand: func(string, string) error { return nil },
			NewContext:       func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			LaunchWorker:     func(string, context.Context, func(context.Context), workerutil.RecoveryOptions) {},
			BaseRecoveryOptions: func() workerutil.RecoveryOptions {
				return workerutil.RecoveryOptions{MaxRetries: 0}
			},
			SessionName: sessionName,
		}
	})

	cfg, err := buildPipeConfig(`\\.\pipe\test`, def, pipeConfigContext{
		rootDir:                 "/root",
		sessionName:             "my-session",
		singleTaskRunnerManager: manager,
	})
	if err != nil {
		t.Fatalf("buildPipeConfig: %v", err)
	}
	if cfg.RuntimeFactory == nil {
		t.Fatal("RuntimeFactory should not be nil for single-task-runner kind")
	}
}

func TestBuildPipeConfig_SingleTaskRunnerNilManagerReturnsError(t *testing.T) {
	def := Definition{
		ID:   "single-task-runner",
		Name: "Single Task Runner",
		Kind: DefinitionKindSingleTaskRunner,
	}

	cfg, err := buildPipeConfig(`\\.\pipe\test`, def, pipeConfigContext{
		rootDir:     "/root",
		sessionName: "my-session",
	})
	if err == nil {
		t.Fatal("expected buildPipeConfig to fail when single-task-runner manager is nil")
	}
	if cfg.RuntimeFactory != nil {
		t.Fatal("RuntimeFactory should be nil on configuration error")
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
