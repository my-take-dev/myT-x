package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/config"
	"myT-x/internal/mcp"
)

func newAppWithMCPManager(t *testing.T, defs ...mcp.MCPDefinition) *App {
	t.Helper()

	registry := mcp.NewRegistry()
	for _, def := range defs {
		if strings.TrimSpace(def.Command) == "" {
			def.Command = "test-command"
		}
		if err := registry.Register(def); err != nil {
			t.Fatalf("Register(%q) error = %v", def.ID, err)
		}
	}

	app := NewApp()
	app.mcpManager = mcp.NewManager(registry, func(string, any) {})
	return app
}

func TestListMCPServers_NilManager(t *testing.T) {
	app := NewApp()

	_, err := app.ListMCPServers("session-a")
	if err == nil {
		t.Fatal("ListMCPServers() expected error when manager is nil")
	}
	if !errors.Is(err, errMCPManagerNotInitialized) {
		t.Fatalf("ListMCPServers() error = %v, want wrapped errMCPManagerNotInitialized", err)
	}
}

func TestListMCPServers_DefaultEnabledSnapshot(t *testing.T) {
	app := newAppWithMCPManager(t, mcp.MCPDefinition{
		ID:             "memory",
		Name:           "Memory Server",
		DefaultEnabled: true,
	})

	snapshots, err := app.ListMCPServers("session-a")
	if err != nil {
		t.Fatalf("ListMCPServers() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("ListMCPServers() snapshots length = %d, want 1", len(snapshots))
	}
	if !snapshots[0].Enabled {
		t.Fatal("ListMCPServers() default enabled snapshot = false, want true")
	}
	if snapshots[0].Status != mcp.StatusStopped {
		t.Fatalf("ListMCPServers() status = %q, want %q", snapshots[0].Status, mcp.StatusStopped)
	}
}

func TestToggleMCPServer_EmptySessionName(t *testing.T) {
	app := newAppWithMCPManager(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})

	if err := app.ToggleMCPServer("   ", "memory", true); err == nil {
		t.Fatal("ToggleMCPServer() expected session-name validation error")
	}
}

func TestToggleMCPServer_NilManager(t *testing.T) {
	app := NewApp()
	err := app.ToggleMCPServer("session-a", "memory", true)
	if err == nil {
		t.Fatal("ToggleMCPServer() expected error when manager is nil")
	}
	if !errors.Is(err, errMCPManagerNotInitialized) {
		t.Fatalf("ToggleMCPServer() error = %v, want wrapped errMCPManagerNotInitialized", err)
	}
}

func TestToggleMCPServer_EmptyMCPID(t *testing.T) {
	app := newAppWithMCPManager(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})

	if err := app.ToggleMCPServer("session-a", "   ", true); err == nil {
		t.Fatal("ToggleMCPServer() expected mcp-id validation error")
	}
}

func TestToggleMCPServer_DisablesDefaultEnabledMCP(t *testing.T) {
	app := newAppWithMCPManager(t, mcp.MCPDefinition{
		ID:             "memory",
		Name:           "Memory Server",
		DefaultEnabled: true,
	})

	if err := app.ToggleMCPServer("session-a", "memory", false); err != nil {
		t.Fatalf("ToggleMCPServer(false) error = %v", err)
	}

	detail, err := app.GetMCPDetail("session-a", "memory")
	if err != nil {
		t.Fatalf("GetMCPDetail() error = %v", err)
	}
	if detail.Enabled {
		t.Fatal("GetMCPDetail().Enabled = true, want false after disabling default-enabled MCP")
	}
}

func TestGetMCPDetail_NilManager(t *testing.T) {
	app := NewApp()

	_, err := app.GetMCPDetail("session-a", "memory")
	if err == nil {
		t.Fatal("GetMCPDetail() expected error when manager is nil")
	}
	if !errors.Is(err, errMCPManagerNotInitialized) {
		t.Fatalf("GetMCPDetail() error = %v, want wrapped errMCPManagerNotInitialized", err)
	}
}

func TestMCPServerConfigsToDefinitions(t *testing.T) {
	configs := []config.MCPServerConfig{
		{
			ID:          "memory",
			Name:        "Memory Server",
			Description: "Persisted memory",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/memory-server"},
			Env: map[string]string{
				"MEM_DIR": "/tmp/memory",
			},
			Enabled:     true,
			UsageSample: "remember this",
			ConfigParams: []config.MCPServerConfigParam{
				{
					Key:          "mode",
					Label:        "Mode",
					DefaultValue: "strict",
					Description:  "Execution mode",
				},
			},
		},
	}

	defs := mcpServerConfigsToDefinitions(configs)
	if len(defs) != 1 {
		t.Fatalf("mcpServerConfigsToDefinitions() length = %d, want 1", len(defs))
	}

	want := mcp.MCPDefinition{
		ID:             "memory",
		Name:           "Memory Server",
		Description:    "Persisted memory",
		Command:        "npx",
		Args:           []string{"-y", "@anthropic/memory-server"},
		DefaultEnv:     map[string]string{"MEM_DIR": "/tmp/memory"},
		DefaultEnabled: true,
		UsageSample:    "remember this",
		ConfigParams: []mcp.MCPConfigParam{
			{
				Key:          "mode",
				Label:        "Mode",
				DefaultValue: "strict",
				Description:  "Execution mode",
			},
		},
	}
	if !reflect.DeepEqual(defs[0], want) {
		t.Fatalf("mcpServerConfigsToDefinitions() mismatch\ngot:  %#v\nwant: %#v", defs[0], want)
	}

	configs[0].Args[0] = "changed"
	configs[0].Env["MEM_DIR"] = "/changed"
	if defs[0].Args[0] == "changed" {
		t.Fatal("definition args were aliased to config args")
	}
	if defs[0].DefaultEnv["MEM_DIR"] == "/changed" {
		t.Fatal("definition env was aliased to config env")
	}
	configs[0].ConfigParams[0].Label = "Changed"
	if defs[0].ConfigParams[0].Label == "Changed" {
		t.Fatal("definition config params were aliased to config config_params")
	}
}
