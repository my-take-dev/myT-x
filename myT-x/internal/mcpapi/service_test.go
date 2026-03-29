package mcpapi

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/mcp"
	"myT-x/internal/mcp/lspmcp/lsppkg"
)

// testErrManagerNotReady is a test-local sentinel for RequireMCPManager
// error propagation. The service simply propagates whatever the injected
// function returns.
var testErrManagerNotReady = errors.New("mcp manager is unavailable")

// testErrRegistryNotReady is a test-local sentinel for RequireMCPRegistry
// error propagation.
var testErrRegistryNotReady = errors.New("mcp registry is unavailable")

const testBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

func newTestService(t *testing.T, defs ...mcp.MCPDefinition) (*Service, *mcp.Manager) {
	return newTestServiceWithConfig(t, mcp.ManagerConfig{
		EmitFn: func(string, any) {},
	}, defs...)
}

func newTestServiceWithConfig(t *testing.T, cfg mcp.ManagerConfig, defs ...mcp.MCPDefinition) (*Service, *mcp.Manager) {
	t.Helper()

	cfgCopy := cfg
	registry := cfgCopy.Registry
	if registry == nil {
		registry = mcp.NewRegistry()
	}
	for _, def := range defs {
		if strings.TrimSpace(def.Command) == "" {
			def.Command = "test-command"
		}
		if err := registry.Register(def); err != nil {
			t.Fatalf("Register(%q) error = %v", def.ID, err)
		}
	}

	cfgCopy.Registry = registry
	mgr := mcp.NewManager(cfgCopy)

	svc := NewService(Deps{
		RequireMCPManager:  func() (*mcp.Manager, error) { return mgr, nil },
		RequireMCPRegistry: func() (*mcp.Registry, error) { return registry, nil },
		BridgeCommand:      func() string { return testBridgeCommand },
	})
	return svc, mgr
}

func newNotReadyService() *Service {
	return NewService(Deps{
		RequireMCPManager:  func() (*mcp.Manager, error) { return nil, testErrManagerNotReady },
		RequireMCPRegistry: func() (*mcp.Registry, error) { return nil, testErrRegistryNotReady },
	})
}

func waitForMCPDetailStatus(t *testing.T, mgr *mcp.Manager, events <-chan struct{}, sessionName, mcpID string, want mcp.Status) mcp.MCPSnapshot {
	t.Helper()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		detail, err := mgr.GetDetail(sessionName, mcpID)
		if err != nil {
			t.Fatalf("GetDetail(%q, %q) error = %v", sessionName, mcpID, err)
		}
		if detail.Status == want {
			return detail
		}
		select {
		case <-events:
		case <-timer.C:
			t.Fatalf("GetDetail(%q, %q) status = %q, want %q before timeout", sessionName, mcpID, detail.Status, want)
		}
	}
}

// ---------------------------------------------------------------------------
// NewService
// ---------------------------------------------------------------------------

func TestNewService_PanicsOnNilRequireMCPManager(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewService should panic when RequireMCPManager is nil")
		}
	}()
	NewService(Deps{
		RequireMCPManager:  nil,
		RequireMCPRegistry: func() (*mcp.Registry, error) { return nil, nil },
	})
}

func TestNewService_PanicsOnNilRequireMCPRegistry(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewService should panic when RequireMCPRegistry is nil")
		}
	}()
	NewService(Deps{
		RequireMCPManager:  func() (*mcp.Manager, error) { return nil, nil },
		RequireMCPRegistry: nil,
	})
}

func TestNewService_DefaultsBridgeCommand(t *testing.T) {
	svc := NewService(Deps{
		RequireMCPManager:  func() (*mcp.Manager, error) { return nil, nil },
		RequireMCPRegistry: func() (*mcp.Registry, error) { return nil, nil },
	})
	if got := svc.deps.BridgeCommand(); got != DefaultBridgeCommand {
		t.Fatalf("BridgeCommand() = %q, want %q", got, DefaultBridgeCommand)
	}
}

// ---------------------------------------------------------------------------
// ListMCPServers
// ---------------------------------------------------------------------------

func TestListMCPServers_EmptySessionName(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})
	_, err := svc.ListMCPServers("   ")
	if err == nil {
		t.Fatal("ListMCPServers() expected session-name validation error")
	}
	if !strings.Contains(err.Error(), "session name is required") {
		t.Fatalf("error = %v, want session name required", err)
	}
}

func TestListMCPServers_NotReady(t *testing.T) {
	svc := newNotReadyService()

	_, err := svc.ListMCPServers("session-a")
	if err == nil {
		t.Fatal("ListMCPServers() expected error when manager is not ready")
	}
	if !errors.Is(err, testErrManagerNotReady) {
		t.Fatalf("ListMCPServers() error = %v, want wrapped testErrManagerNotReady", err)
	}
}

func TestListMCPServers_DefaultEnabledSnapshot(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{
		ID:             "memory",
		Name:           "Memory Server",
		DefaultEnabled: true,
	})

	snapshots, err := svc.ListMCPServers("session-a")
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

// ---------------------------------------------------------------------------
// ToggleMCPServer
// ---------------------------------------------------------------------------

func TestToggleMCPServer_EmptySessionName(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})

	if err := svc.ToggleMCPServer("   ", "memory", true); err == nil {
		t.Fatal("ToggleMCPServer() expected session-name validation error")
	}
}

func TestToggleMCPServer_NotReady(t *testing.T) {
	svc := newNotReadyService()
	err := svc.ToggleMCPServer("session-a", "memory", true)
	if err == nil {
		t.Fatal("ToggleMCPServer() expected error when manager is not ready")
	}
	if !errors.Is(err, testErrManagerNotReady) {
		t.Fatalf("ToggleMCPServer() error = %v, want wrapped testErrManagerNotReady", err)
	}
}

func TestToggleMCPServer_EmptyMCPID(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})

	if err := svc.ToggleMCPServer("session-a", "   ", true); err == nil {
		t.Fatal("ToggleMCPServer() expected mcp-id validation error")
	}
}

func TestToggleMCPServer_UnknownMCPID(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})
	err := svc.ToggleMCPServer("session-a", "nonexistent", true)
	if err == nil {
		t.Fatal("ToggleMCPServer() expected error for unknown mcpID")
	}
}

func TestToggleMCPServer_DisablesDefaultEnabledMCP(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{
		ID:             "memory",
		Name:           "Memory Server",
		DefaultEnabled: true,
	})

	if err := svc.ToggleMCPServer("session-a", "memory", false); err != nil {
		t.Fatalf("ToggleMCPServer(false) error = %v", err)
	}

	detail, err := svc.GetMCPDetail("session-a", "memory")
	if err != nil {
		t.Fatalf("GetMCPDetail() error = %v", err)
	}
	if detail.Enabled {
		t.Fatal("GetMCPDetail().Enabled = true, want false after disabling default-enabled MCP")
	}
}

// ---------------------------------------------------------------------------
// GetMCPDetail
// ---------------------------------------------------------------------------

func TestGetMCPDetail_EmptySessionName(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})
	_, err := svc.GetMCPDetail("   ", "memory")
	if err == nil {
		t.Fatal("GetMCPDetail() expected session-name validation error")
	}
	if !strings.Contains(err.Error(), "session name is required") {
		t.Fatalf("error = %v, want session name required", err)
	}
}

func TestGetMCPDetail_EmptyMCPID(t *testing.T) {
	svc, _ := newTestService(t, mcp.MCPDefinition{ID: "memory", Name: "Memory Server"})
	_, err := svc.GetMCPDetail("session-a", "   ")
	if err == nil {
		t.Fatal("GetMCPDetail() expected mcp-id validation error")
	}
	if !strings.Contains(err.Error(), "mcp ID is required") {
		t.Fatalf("error = %v, want mcp ID required", err)
	}
}

func TestGetMCPDetail_NotReady(t *testing.T) {
	svc := newNotReadyService()

	_, err := svc.GetMCPDetail("session-a", "memory")
	if err == nil {
		t.Fatal("GetMCPDetail() expected error when manager is not ready")
	}
	if !errors.Is(err, testErrManagerNotReady) {
		t.Fatalf("GetMCPDetail() error = %v, want wrapped testErrManagerNotReady", err)
	}
}

// ---------------------------------------------------------------------------
// applyBridgeRecommendation
// ---------------------------------------------------------------------------

func TestApplyBridgeRecommendation_RunningUsesMyTXCommand(t *testing.T) {
	svc, _ := newTestService(t)

	snapshot := mcp.MCPSnapshot{
		ID:       "lsp-gopls",
		Status:   mcp.StatusRunning,
		PipePath: `\\.\pipe\myT-x-mcp-user-session-lsp-gopls`,
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != testBridgeCommand {
		t.Fatalf("BridgeCommand = %q, want myT-x.exe path", snapshot.BridgeCommand)
	}
	wantArgs := []string{
		"mcp",
		"stdio",
		"--mcp",
		"gopls",
	}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyBridgeRecommendation_StoppedStillProvidesLaunchRecommendation(t *testing.T) {
	svc, _ := newTestService(t)

	snapshot := mcp.MCPSnapshot{
		ID:            "lsp-gopls",
		Status:        mcp.StatusStopped,
		PipePath:      `\\.\pipe\example`,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != testBridgeCommand {
		t.Fatalf("BridgeCommand = %q, want myT-x.exe path", snapshot.BridgeCommand)
	}
	wantArgs := []string{
		"mcp",
		"stdio",
		"--mcp",
		"gopls",
	}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyBridgeRecommendation_ErrorStillProvidesLaunchRecommendation(t *testing.T) {
	svc, _ := newTestService(t)

	snapshot := mcp.MCPSnapshot{
		ID:     "lsp-gopls",
		Status: mcp.StatusError,
		Error:  "startup failed",
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != testBridgeCommand {
		t.Fatalf("BridgeCommand = %q, want myT-x.exe path", snapshot.BridgeCommand)
	}
	wantArgs := []string{
		"mcp",
		"stdio",
		"--mcp",
		"gopls",
	}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyBridgeRecommendation_OrchestratorIDGeneratesRecommendation(t *testing.T) {
	svc, _ := newTestService(t)
	snapshot := mcp.MCPSnapshot{
		ID:     "orch-agent-orchestrator",
		Status: mcp.StatusRunning,
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != testBridgeCommand {
		t.Fatalf("BridgeCommand = %q, want %q", snapshot.BridgeCommand, testBridgeCommand)
	}
	wantArgs := []string{"mcp", "stdio", "--mcp", "agent-orchestrator"}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyBridgeRecommendation_InvalidOrchIDLeavesEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	snapshot := mcp.MCPSnapshot{
		ID:            "orch-",
		Status:        mcp.StatusRunning,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty for bare orch- prefix", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyBridgeRecommendation_LeavesRecommendationEmptyForBlankSession(t *testing.T) {
	svc, _ := newTestService(t)

	snapshot := mcp.MCPSnapshot{
		ID:     "lsp-gopls",
		Status: mcp.StatusRunning,
	}
	svc.applyBridgeRecommendation("   ", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyBridgeRecommendation_LeavesRecommendationEmptyForNonLSPID(t *testing.T) {
	svc, _ := newTestService(t)

	snapshot := mcp.MCPSnapshot{
		ID:            "memory",
		Status:        mcp.StatusStopped,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyBridgeRecommendation_LeavesRecommendationEmptyForInvalidLSPID(t *testing.T) {
	svc, _ := newTestService(t)

	snapshot := mcp.MCPSnapshot{
		ID:            "lsp-",
		Status:        mcp.StatusStopped,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	svc.applyBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyBridgeRecommendation_NormalizesVariantLSPIDs(t *testing.T) {
	svc, _ := newTestService(t)

	tests := []struct {
		name    string
		mcpID   string
		wantMCP string
	}{
		{
			name:    "multi-word id keeps hyphenated suffix",
			mcpID:   "lsp-rust-analyzer",
			wantMCP: "rust-analyzer",
		},
		{
			name:    "mixed case prefix is normalized",
			mcpID:   "LSP-Pyright",
			wantMCP: "pyright",
		},
		{
			name:    "command-style suffixes are stripped",
			mcpID:   "lsp-cmd.COM.exe",
			wantMCP: "cmd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := mcp.MCPSnapshot{
				ID:     tt.mcpID,
				Status: mcp.StatusStopped,
			}
			svc.applyBridgeRecommendation("session-a", &snapshot)

			wantArgs := []string{
				"mcp",
				"stdio",
				"--mcp",
				tt.wantMCP,
			}
			if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
				t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveBridgeCommand
// ---------------------------------------------------------------------------

func TestResolveBridgeCommand_ReturnsExecutablePath(t *testing.T) {
	t.Parallel()
	got := resolveBridgeCommandWith(func() (string, error) {
		return `C:\Program Files\myT-x\myT-x.exe`, nil
	})
	if got != `C:\Program Files\myT-x\myT-x.exe` {
		t.Fatalf("resolveBridgeCommandWith() = %q, want executable path", got)
	}
}

func TestResolveBridgeCommand_FallbackOnExecutableError(t *testing.T) {
	t.Parallel()
	got := resolveBridgeCommandWith(func() (string, error) {
		return "", errors.New("boom")
	})
	if got != DefaultBridgeCommand {
		t.Fatalf("resolveBridgeCommandWith() = %q, want %q", got, DefaultBridgeCommand)
	}
}

func TestResolveBridgeCommand_FallbackOnEmptyExecutablePath(t *testing.T) {
	t.Parallel()
	got := resolveBridgeCommandWith(func() (string, error) {
		return "   ", nil
	})
	if got != DefaultBridgeCommand {
		t.Fatalf("resolveBridgeCommandWith() = %q, want %q", got, DefaultBridgeCommand)
	}
}

// ---------------------------------------------------------------------------
// resolveMCPIDForCLIName
// ---------------------------------------------------------------------------

func TestResolveMCPIDForCLIName_Alias(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
		mcp.MCPDefinition{ID: "memory", Name: "Memory", Command: "npx"},
	)

	mcpID, err := svc.resolveMCPIDForCLIName("gopls")
	if err != nil {
		t.Fatalf("resolveMCPIDForCLIName(gopls) error = %v", err)
	}
	if mcpID != "lsp-gopls" {
		t.Fatalf("resolveMCPIDForCLIName(gopls) = %q, want %q", mcpID, "lsp-gopls")
	}
}

func TestResolveMCPIDForCLIName_DisplayNameAlias(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	mcpID, err := svc.resolveMCPIDForCLIName("Go LSP")
	if err != nil {
		t.Fatalf("resolveMCPIDForCLIName(Go LSP) error = %v", err)
	}
	if mcpID != "lsp-gopls" {
		t.Fatalf("resolveMCPIDForCLIName(Go LSP) = %q, want %q", mcpID, "lsp-gopls")
	}
}

func TestResolveMCPIDForCLIName_AmbiguousAlias(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "custom-a", Name: "Custom A", Command: "foo"},
		mcp.MCPDefinition{ID: "custom-b", Name: "Custom B", Command: "foo"},
	)

	_, err := svc.resolveMCPIDForCLIName("foo")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName(foo) should fail with ambiguity")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v, want ambiguous message", err)
	}
}

func TestResolveMCPIDForCLIName_ExactIDMatch(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	mcpID, err := svc.resolveMCPIDForCLIName("lsp-gopls")
	if err != nil {
		t.Fatalf("resolveMCPIDForCLIName(lsp-gopls) error = %v", err)
	}
	if mcpID != "lsp-gopls" {
		t.Fatalf("resolveMCPIDForCLIName(lsp-gopls) = %q, want %q", mcpID, "lsp-gopls")
	}
}

func TestResolveMCPIDForCLIName_EmptyInput(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	_, err := svc.resolveMCPIDForCLIName("   ")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should reject empty input")
	}
	if !strings.Contains(err.Error(), "mcp name is required") {
		t.Fatalf("error = %v, want empty input validation", err)
	}
}

func TestResolveMCPIDForCLIName_RegistryNotReady(t *testing.T) {
	svc := newNotReadyService()

	_, err := svc.resolveMCPIDForCLIName("gopls")
	if !errors.Is(err, testErrRegistryNotReady) {
		t.Fatalf("resolveMCPIDForCLIName error = %v, want %v", err, testErrRegistryNotReady)
	}
}

func TestResolveMCPIDForCLIName_NoDefinitions(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.resolveMCPIDForCLIName("gopls")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should fail when no definitions are registered")
	}
	if !strings.Contains(err.Error(), "no mcp definitions are registered") {
		t.Fatalf("error = %v, want empty registry message", err)
	}
}

func TestResolveMCPIDForCLIName_GenericLauncherExcluded(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "memory", Name: "Memory", Command: "npx"},
	)

	_, err := svc.resolveMCPIDForCLIName("npx")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should not resolve generic launcher aliases")
	}
	if !strings.Contains(err.Error(), "unknown mcp") {
		t.Fatalf("error = %v, want unknown mcp error", err)
	}
}

func TestResolveMCPIDForCLIName_DenoLauncherExcluded(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "deno-mcp", Name: "Deno MCP", Command: "deno"},
	)

	_, err := svc.resolveMCPIDForCLIName("deno")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should not resolve deno as a generic launcher alias")
	}
	if !strings.Contains(err.Error(), "unknown mcp") {
		t.Fatalf("error = %v, want unknown mcp error", err)
	}
}

func TestResolveMCPIDForCLIName_CaseInsensitiveInputs(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	tests := []struct {
		name  string
		input string
	}{
		{name: "exact id equal fold", input: "LSP-GOPLS"},
		{name: "normalized alias uppercase", input: "GOPLS"},
		{name: "normalized alias executable suffix", input: "Gopls.EXE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpID, err := svc.resolveMCPIDForCLIName(tt.input)
			if err != nil {
				t.Fatalf("resolveMCPIDForCLIName(%q) error = %v", tt.input, err)
			}
			if mcpID != "lsp-gopls" {
				t.Fatalf("resolveMCPIDForCLIName(%q) = %q, want %q", tt.input, mcpID, "lsp-gopls")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveMCPStdio
// ---------------------------------------------------------------------------

func TestResolveMCPStdio_UsesAliasAndReturnsPipe(t *testing.T) {
	svc, mgr := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	t.Cleanup(func() {
		mgr.Close()
	})

	sessionName := "session-stdio-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")
	resolved, err := svc.ResolveMCPStdio(sessionName, "gopls")
	if err != nil {
		t.Fatalf("ResolveMCPStdio error = %v", err)
	}
	if resolved.MCPID != "lsp-gopls" {
		t.Fatalf("MCPID = %q, want %q", resolved.MCPID, "lsp-gopls")
	}
	wantPipe := mcp.BuildMCPPipeName(sessionName, "lsp-gopls")
	if resolved.PipePath != wantPipe {
		t.Fatalf("PipePath = %q, want %q", resolved.PipePath, wantPipe)
	}
	detail, err := mgr.GetDetail(sessionName, "lsp-gopls")
	if err != nil {
		t.Fatalf("GetDetail error = %v", err)
	}
	if !detail.Enabled {
		t.Fatal("detail.Enabled should be true after ResolveMCPStdio")
	}
}

func TestResolveMCPStdio_StatusStartingFallsBackToDeterministicPipe(t *testing.T) {
	t.Parallel()

	workDirStarted := make(chan struct{})
	releaseWorkDir := make(chan struct{})
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		close(releaseWorkDir)
	}

	svc, mgr := newTestServiceWithConfig(t, mcp.ManagerConfig{
		EmitFn: func(string, any) {},
		ResolveWorkDir: func(string) (string, error) {
			close(workDirStarted)
			<-releaseWorkDir
			return t.TempDir(), nil
		},
	}, mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"})
	svc.deps.ReadinessWaitTimeout = 50 * time.Millisecond
	svc.deps.ReadinessWaitInterval = 10 * time.Millisecond
	t.Cleanup(func() {
		release()
		mgr.Close()
	})

	sessionName := "session-starting"
	resolved, err := svc.ResolveMCPStdio(sessionName, "gopls")
	if err != nil {
		t.Fatalf("ResolveMCPStdio error = %v", err)
	}

	select {
	case <-workDirStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("ResolveWorkDir was not called")
	}

	wantPipe := mcp.BuildMCPPipeName(sessionName, "lsp-gopls")
	if resolved.PipePath != wantPipe {
		t.Fatalf("PipePath = %q, want %q while status is starting", resolved.PipePath, wantPipe)
	}

	detail, err := mgr.GetDetail(sessionName, "lsp-gopls")
	if err != nil {
		t.Fatalf("GetDetail error = %v", err)
	}
	if detail.Status != mcp.StatusStarting {
		t.Fatalf("detail.Status = %q, want %q", detail.Status, mcp.StatusStarting)
	}

	release()
}

func TestResolveMCPStdio_ReadinessWaitTransitionsToRunning(t *testing.T) {
	t.Parallel()

	// ResolveWorkDir completes after 200ms, so startInstance transitions to Running.
	svc, mgr := newTestServiceWithConfig(t, mcp.ManagerConfig{
		EmitFn: func(string, any) {},
		ResolveWorkDir: func(string) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return t.TempDir(), nil
		},
	}, mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"})
	svc.deps.ReadinessWaitTimeout = 3 * time.Second
	svc.deps.ReadinessWaitInterval = 50 * time.Millisecond
	t.Cleanup(func() {
		mgr.Close()
	})

	sessionName := "session-readiness-running"
	resolved, err := svc.ResolveMCPStdio(sessionName, "gopls")
	if err != nil {
		t.Fatalf("ResolveMCPStdio error = %v", err)
	}

	wantPipe := mcp.BuildMCPPipeName(sessionName, "lsp-gopls")
	if resolved.PipePath != wantPipe {
		t.Fatalf("PipePath = %q, want %q", resolved.PipePath, wantPipe)
	}
}

func TestResolveMCPStdio_ReadinessWaitTimesOutReturnsStarting(t *testing.T) {
	t.Parallel()

	releaseWorkDir := make(chan struct{})
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		close(releaseWorkDir)
	}

	// ResolveWorkDir blocks indefinitely, so status stays Starting.
	svc, mgr := newTestServiceWithConfig(t, mcp.ManagerConfig{
		EmitFn: func(string, any) {},
		ResolveWorkDir: func(string) (string, error) {
			<-releaseWorkDir
			return t.TempDir(), nil
		},
	}, mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"})
	svc.deps.ReadinessWaitTimeout = 50 * time.Millisecond
	svc.deps.ReadinessWaitInterval = 10 * time.Millisecond
	t.Cleanup(func() {
		release()
		mgr.Close()
	})

	sessionName := "session-readiness-timeout"
	resolved, err := svc.ResolveMCPStdio(sessionName, "gopls")
	if err != nil {
		t.Fatalf("ResolveMCPStdio error = %v, want success with StatusStarting fallback", err)
	}

	wantPipe := mcp.BuildMCPPipeName(sessionName, "lsp-gopls")
	if resolved.PipePath != wantPipe {
		t.Fatalf("PipePath = %q, want %q", resolved.PipePath, wantPipe)
	}

	detail, err := mgr.GetDetail(sessionName, "lsp-gopls")
	if err != nil {
		t.Fatalf("GetDetail error = %v", err)
	}
	if detail.Status != mcp.StatusStarting {
		t.Fatalf("detail.Status = %q, want %q after readiness wait timeout", detail.Status, mcp.StatusStarting)
	}

	release()
}

func TestResolveMCPStdio_EmptyInputs(t *testing.T) {
	svc, _ := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	_, err := svc.ResolveMCPStdio("   ", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should reject empty session name")
	}
	if !strings.Contains(err.Error(), "session name is required") {
		t.Fatalf("session validation error = %v", err)
	}

	_, err = svc.ResolveMCPStdio("session-a", "   ")
	if err == nil {
		t.Fatal("ResolveMCPStdio should reject empty mcp name")
	}
	if !strings.Contains(err.Error(), "mcp name is required") {
		t.Fatalf("mcp validation error = %v", err)
	}
}

func TestResolveMCPStdio_NotReady(t *testing.T) {
	svc := newNotReadyService()

	_, err := svc.ResolveMCPStdio("session-a", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should fail when manager is not ready")
	}
	if !errors.Is(err, testErrManagerNotReady) {
		t.Fatalf("ResolveMCPStdio error = %v, want %v", err, testErrManagerNotReady)
	}
}

func TestResolveMCPStdio_StatusStopped(t *testing.T) {
	svc, mgr := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls", DefaultEnabled: true},
	)
	t.Cleanup(func() {
		mgr.Close()
	})

	_, err := svc.ResolveMCPStdio("session-stopped", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should fail when the MCP remains stopped")
	}
	if !strings.Contains(err.Error(), "was stopped before becoming ready") {
		t.Fatalf("error = %v, want stopped-before-ready message", err)
	}
}

func TestResolveMCPStdio_StatusError(t *testing.T) {
	stateChanged := make(chan struct{}, 4)
	svc, mgr := newTestServiceWithConfig(t, mcp.ManagerConfig{
		EmitFn: func(name string, payload any) {
			if name != "mcp:state-changed" {
				return
			}
			event, ok := payload.(map[string]any)
			if !ok {
				return
			}
			if event["session_name"] != "session-error" || event["mcp_id"] != "lsp-gopls" {
				return
			}
			select {
			case stateChanged <- struct{}{}:
			default:
			}
		},
		ResolveWorkDir: func(string) (string, error) {
			return "", errors.New("workdir failed")
		},
	}, mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"})
	t.Cleanup(func() {
		mgr.Close()
	})

	if err := mgr.SetEnabled("session-error", "lsp-gopls", true); err != nil {
		t.Fatalf("SetEnabled error = %v", err)
	}
	detail := waitForMCPDetailStatus(t, mgr, stateChanged, "session-error", "lsp-gopls", mcp.StatusError)
	if !detail.Enabled {
		t.Fatal("detail.Enabled = false, want true after failed startup")
	}

	_, err := svc.ResolveMCPStdio("session-error", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should fail when the MCP is already in error state")
	}
	if !strings.Contains(err.Error(), "failed to start") {
		t.Fatalf("error = %v, want failed-to-start message", err)
	}
}

// ---------------------------------------------------------------------------
// rollbackResolvedMCP
// ---------------------------------------------------------------------------

func TestRollbackResolvedMCP_DisablesOnlyNewlyEnabledMCP(t *testing.T) {
	_, mgr := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	t.Cleanup(func() {
		mgr.Close()
	})

	sessionName := "session-rollback-disable"
	if err := mgr.SetEnabled(sessionName, "lsp-gopls", true); err != nil {
		t.Fatalf("SetEnabled error = %v", err)
	}

	cause := errors.New("startup failed")
	err := rollbackResolvedMCP(mgr, sessionName, "lsp-gopls", true, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("rollbackResolvedMCP error = %v, want wrapped cause %v", err, cause)
	}

	detail, detailErr := mgr.GetDetail(sessionName, "lsp-gopls")
	if detailErr != nil {
		t.Fatalf("GetDetail error = %v", detailErr)
	}
	if detail.Enabled {
		t.Fatal("detail.Enabled = true, want false after rollback")
	}
}

func TestRollbackResolvedMCP_PreservesPreviouslyEnabledMCP(t *testing.T) {
	_, mgr := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	t.Cleanup(func() {
		mgr.Close()
	})

	sessionName := "session-rollback-preserve"
	if err := mgr.SetEnabled(sessionName, "lsp-gopls", true); err != nil {
		t.Fatalf("SetEnabled error = %v", err)
	}

	cause := errors.New("startup failed")
	err := rollbackResolvedMCP(mgr, sessionName, "lsp-gopls", false, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("rollbackResolvedMCP error = %v, want wrapped cause %v", err, cause)
	}

	detail, detailErr := mgr.GetDetail(sessionName, "lsp-gopls")
	if detailErr != nil {
		t.Fatalf("GetDetail error = %v", detailErr)
	}
	if !detail.Enabled {
		t.Fatal("detail.Enabled = false, want true when rollback is skipped")
	}
}

func TestRollbackResolvedMCP_JoinsRollbackFailure(t *testing.T) {
	_, mgr := newTestService(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	sessionName := "session-rollback-join"
	cause := errors.New("startup failed")

	mgr.Close()
	err := rollbackResolvedMCP(mgr, sessionName, "lsp-gopls", true, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("rollbackResolvedMCP error = %v, want wrapped cause %v", err, cause)
	}
	if err == nil || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("rollbackResolvedMCP error = %v, want rollback failure detail", err)
	}
}

func TestRollbackResolvedMCP_NilManagerReturnsCause(t *testing.T) {
	cause := errors.New("startup failed")

	err := rollbackResolvedMCP(nil, "session-a", "lsp-gopls", true, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("rollbackResolvedMCP error = %v, want wrapped cause %v", err, cause)
	}
}

// ---------------------------------------------------------------------------
// normalizeMCPAliasToken
// ---------------------------------------------------------------------------

func TestNormalizeMCPAliasToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "   ", want: ""},
		{name: "quoted executable", input: `"gopls.exe"`, want: "gopls"},
		{name: "cmd chained extension", input: `C:\Windows\System32\cmd.COM.exe`, want: "cmd"},
		{name: "batch file", input: `C:\tools\my-tool.bat`, want: "my-tool"},
		{name: "plain alias", input: "Go LSP", want: "go lsp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMCPAliasToken(tt.input); got != tt.want {
				t.Fatalf("normalizeMCPAliasToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lspMCPCLINameFromID / orchMCPCLINameFromID
// ---------------------------------------------------------------------------

func TestLSPMCPCLINameFromID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "mixed case prefix", input: "LSP-Gopls", want: "gopls"},
		{name: "legacy com suffix", input: "lsp-cmd.COM.exe", want: "cmd"},
		{name: "bare lsp prefix", input: "lsp-", want: ""},
		{name: "non lsp id", input: "memory", want: ""},
		{name: "blank input", input: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lspMCPCLINameFromID(tt.input); got != tt.want {
				t.Fatalf("lspMCPCLINameFromID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOrchMCPCLINameFromID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "valid orch id", input: "orch-agent-orchestrator", want: "agent-orchestrator"},
		{name: "mixed case prefix", input: "ORCH-MyAgent", want: "myagent"},
		{name: "bare orch prefix", input: "orch-", want: ""},
		{name: "non orch id", input: "memory", want: ""},
		{name: "lsp id", input: "lsp-gopls", want: ""},
		{name: "blank input", input: "   ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orchMCPCLINameFromID(tt.input); got != tt.want {
				t.Fatalf("orchMCPCLINameFromID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// addMCPAlias / sortedAliasCandidates
// ---------------------------------------------------------------------------

func TestAddMCPAlias_DeduplicatesNormalizedAlias(t *testing.T) {
	aliasToIDs := map[string]map[string]struct{}{}

	addMCPAlias(aliasToIDs, `C:\Tools\gopls.exe`, "lsp-gopls")
	addMCPAlias(aliasToIDs, "gopls", "lsp-gopls")

	candidates := sortedAliasCandidates(aliasToIDs["gopls"])
	if !reflect.DeepEqual(candidates, []string{"lsp-gopls"}) {
		t.Fatalf("sortedAliasCandidates(gopls) = %#v, want %#v", candidates, []string{"lsp-gopls"})
	}
}

func TestSortedAliasCandidates_ReturnsSortedIDs(t *testing.T) {
	got := sortedAliasCandidates(map[string]struct{}{
		"custom-b": {},
		"custom-a": {},
	})
	want := []string{"custom-a", "custom-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedAliasCandidates() = %#v, want %#v", got, want)
	}
}

// ---------------------------------------------------------------------------
// MCPServerConfigsToDefinitions
// ---------------------------------------------------------------------------

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

	defs := MCPServerConfigsToDefinitions(configs)
	if len(defs) != 1 {
		t.Fatalf("MCPServerConfigsToDefinitions() length = %d, want 1", len(defs))
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
		t.Fatalf("MCPServerConfigsToDefinitions() mismatch\ngot:  %#v\nwant: %#v", defs[0], want)
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

func TestMCPServerConfigsToDefinitions_Empty(t *testing.T) {
	if defs := MCPServerConfigsToDefinitions(nil); defs != nil {
		t.Fatalf("MCPServerConfigsToDefinitions(nil) = %v, want nil", defs)
	}
	if defs := MCPServerConfigsToDefinitions([]config.MCPServerConfig{}); defs != nil {
		t.Fatalf("MCPServerConfigsToDefinitions([]) = %v, want nil", defs)
	}
}

// ---------------------------------------------------------------------------
// LSPExtensionMetaToDefinitions
// ---------------------------------------------------------------------------

func TestLSPExtensionMetaToDefinitions(t *testing.T) {
	metas := []lsppkg.ExtensionMeta{
		{Name: "gopls", Language: "Go", DefaultCommand: "gopls"},
		{Name: "rust", Language: "Rust", DefaultCommand: "rust-analyzer"},
	}
	defs := LSPExtensionMetaToDefinitions(metas)
	if len(defs) != 2 {
		t.Fatalf("LSPExtensionMetaToDefinitions() len = %d, want 2", len(defs))
	}
	if defs[0].ID != "lsp-gopls" {
		t.Errorf("ID = %q, want %q", defs[0].ID, "lsp-gopls")
	}
	if defs[0].Name != "Go (LSP: gopls)" {
		t.Errorf("Name = %q, want %q", defs[0].Name, "Go (LSP: gopls)")
	}
	if defs[0].Command != "gopls" {
		t.Errorf("Command = %q, want %q", defs[0].Command, "gopls")
	}
	if defs[1].ID != "lsp-rust" {
		t.Errorf("ID = %q, want %q", defs[1].ID, "lsp-rust")
	}
	for _, d := range defs {
		if d.DefaultEnabled {
			t.Errorf("DefaultEnabled = true for %q, want false", d.ID)
		}
	}
}

func TestLSPExtensionMetaToDefinitions_Empty(t *testing.T) {
	defs := LSPExtensionMetaToDefinitions(nil)
	if defs != nil {
		t.Fatalf("LSPExtensionMetaToDefinitions(nil) = %v, want nil", defs)
	}
}

func TestLSPExtensionMetaToDefinitions_IntegrationWithAllExtensionMeta(t *testing.T) {
	metas := lsppkg.AllExtensionMeta()
	defs := LSPExtensionMetaToDefinitions(metas)
	if len(defs) == 0 {
		t.Fatal("LSPExtensionMetaToDefinitions(AllExtensionMeta()) returned empty list")
	}
	// Verify all definitions can be registered in a fresh registry.
	registry := mcp.NewRegistry()
	for _, def := range defs {
		if err := registry.Register(def); err != nil {
			t.Errorf("Registry.Register(%q) error = %v", def.ID, err)
		}
	}
	all := registry.All()
	if len(all) != len(defs) {
		t.Errorf("registry.All() len = %d, want %d", len(all), len(defs))
	}
}

// ---------------------------------------------------------------------------
// Field count guard tests
// ---------------------------------------------------------------------------

func TestDepsFieldCount(t *testing.T) {
	t.Parallel()
	const expectedFields = 5
	actual := reflect.TypeFor[Deps]().NumField()
	if actual != expectedFields {
		t.Fatalf("Deps has %d fields, expected %d — update tests and NewService validation for new fields", actual, expectedFields)
	}
}
