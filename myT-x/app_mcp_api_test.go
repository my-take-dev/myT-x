package main

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

func newAppWithMCPManager(t *testing.T, defs ...mcp.MCPDefinition) *App {
	return newAppWithMCPManagerConfig(t, mcp.ManagerConfig{
		EmitFn: func(string, any) {},
	}, defs...)
}

func newAppWithMCPManagerConfig(t *testing.T, cfg mcp.ManagerConfig, defs ...mcp.MCPDefinition) *App {
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

	app := NewApp()
	app.mcpRegistry = registry
	cfgCopy.Registry = registry
	app.mcpManager = mcp.NewManager(cfgCopy)
	return app
}

func waitForMCPDetailStatusEvent(t *testing.T, app *App, events <-chan struct{}, sessionName, mcpID string, want mcp.Status) mcp.MCPSnapshot {
	t.Helper()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		detail, err := app.mcpManager.GetDetail(sessionName, mcpID)
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

func TestApplyMCPBridgeRecommendation_RunningUsesMyTXCommand(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

	snapshot := mcp.MCPSnapshot{
		ID:       "lsp-gopls",
		Status:   mcp.StatusRunning,
		PipePath: `\\.\pipe\myT-x-mcp-user-session-lsp-gopls`,
	}
	app.applyMCPBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != `C:\Program Files\myT-x\myT-x.exe` {
		t.Fatalf("BridgeCommand = %q, want myT-x.exe path", snapshot.BridgeCommand)
	}
	wantArgs := []string{
		"mcp",
		"stdio",
		"--session",
		"session-a",
		"--mcp",
		"gopls",
	}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyMCPBridgeRecommendation_StoppedStillProvidesLaunchRecommendation(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

	snapshot := mcp.MCPSnapshot{
		ID:            "lsp-gopls",
		Status:        mcp.StatusStopped,
		PipePath:      `\\.\pipe\example`,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	app.applyMCPBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != `C:\Program Files\myT-x\myT-x.exe` {
		t.Fatalf("BridgeCommand = %q, want myT-x.exe path", snapshot.BridgeCommand)
	}
	wantArgs := []string{
		"mcp",
		"stdio",
		"--session",
		"session-a",
		"--mcp",
		"gopls",
	}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyMCPBridgeRecommendation_ErrorStillProvidesLaunchRecommendation(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

	snapshot := mcp.MCPSnapshot{
		ID:     "lsp-gopls",
		Status: mcp.StatusError,
		Error:  "startup failed",
	}
	app.applyMCPBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != `C:\Program Files\myT-x\myT-x.exe` {
		t.Fatalf("BridgeCommand = %q, want myT-x.exe path", snapshot.BridgeCommand)
	}
	wantArgs := []string{
		"mcp",
		"stdio",
		"--session",
		"session-a",
		"--mcp",
		"gopls",
	}
	if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
		t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
	}
}

func TestApplyMCPBridgeRecommendation_LeavesRecommendationEmptyForBlankSession(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

	snapshot := mcp.MCPSnapshot{
		ID:     "lsp-gopls",
		Status: mcp.StatusRunning,
	}
	app.applyMCPBridgeRecommendation("   ", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyMCPBridgeRecommendation_LeavesRecommendationEmptyForNonLSPID(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

	snapshot := mcp.MCPSnapshot{
		ID:            "memory",
		Status:        mcp.StatusStopped,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	app.applyMCPBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyMCPBridgeRecommendation_LeavesRecommendationEmptyForInvalidLSPID(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

	snapshot := mcp.MCPSnapshot{
		ID:            "lsp-",
		Status:        mcp.StatusStopped,
		BridgeCommand: "stale.exe",
		BridgeArgs:    []string{"stale"},
	}
	app.applyMCPBridgeRecommendation("session-a", &snapshot)

	if snapshot.BridgeCommand != "" {
		t.Fatalf("BridgeCommand = %q, want empty", snapshot.BridgeCommand)
	}
	if snapshot.BridgeArgs != nil {
		t.Fatalf("BridgeArgs = %#v, want nil", snapshot.BridgeArgs)
	}
}

func TestApplyMCPBridgeRecommendation_NormalizesVariantLSPIDs(t *testing.T) {
	app := NewApp()
	app.mcpBridgeCommand = `C:\Program Files\myT-x\myT-x.exe`

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
			app.applyMCPBridgeRecommendation("session-a", &snapshot)

			wantArgs := []string{
				"mcp",
				"stdio",
				"--session",
				"session-a",
				"--mcp",
				tt.wantMCP,
			}
			if !reflect.DeepEqual(snapshot.BridgeArgs, wantArgs) {
				t.Fatalf("BridgeArgs = %#v, want %#v", snapshot.BridgeArgs, wantArgs)
			}
		})
	}
}

func TestResolveMCPBridgeCommand_FallbackOnExecutableError(t *testing.T) {
	original := osExecutableFn
	t.Cleanup(func() {
		osExecutableFn = original
	})
	osExecutableFn = func() (string, error) {
		return "", errors.New("boom")
	}

	if got := resolveMCPBridgeCommand(); got != defaultMCPBridgeCommand {
		t.Fatalf("resolveMCPBridgeCommand() = %q, want %q", got, defaultMCPBridgeCommand)
	}
}

func TestResolveMCPBridgeCommand_FallbackOnEmptyExecutablePath(t *testing.T) {
	original := osExecutableFn
	t.Cleanup(func() {
		osExecutableFn = original
	})
	osExecutableFn = func() (string, error) {
		return "   ", nil
	}

	if got := resolveMCPBridgeCommand(); got != defaultMCPBridgeCommand {
		t.Fatalf("resolveMCPBridgeCommand() = %q, want %q", got, defaultMCPBridgeCommand)
	}
}

func TestResolveMCPIDForCLIName_Alias(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
		mcp.MCPDefinition{ID: "memory", Name: "Memory", Command: "npx"},
	)

	mcpID, err := app.resolveMCPIDForCLIName("gopls")
	if err != nil {
		t.Fatalf("resolveMCPIDForCLIName(gopls) error = %v", err)
	}
	if mcpID != "lsp-gopls" {
		t.Fatalf("resolveMCPIDForCLIName(gopls) = %q, want %q", mcpID, "lsp-gopls")
	}
}

func TestResolveMCPIDForCLIName_DisplayNameAlias(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	mcpID, err := app.resolveMCPIDForCLIName("Go LSP")
	if err != nil {
		t.Fatalf("resolveMCPIDForCLIName(Go LSP) error = %v", err)
	}
	if mcpID != "lsp-gopls" {
		t.Fatalf("resolveMCPIDForCLIName(Go LSP) = %q, want %q", mcpID, "lsp-gopls")
	}
}

func TestResolveMCPIDForCLIName_AmbiguousAlias(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "custom-a", Name: "Custom A", Command: "foo"},
		mcp.MCPDefinition{ID: "custom-b", Name: "Custom B", Command: "foo"},
	)

	_, err := app.resolveMCPIDForCLIName("foo")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName(foo) should fail with ambiguity")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v, want ambiguous message", err)
	}
}

func TestResolveMCPIDForCLIName_ExactIDMatch(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	mcpID, err := app.resolveMCPIDForCLIName("lsp-gopls")
	if err != nil {
		t.Fatalf("resolveMCPIDForCLIName(lsp-gopls) error = %v", err)
	}
	if mcpID != "lsp-gopls" {
		t.Fatalf("resolveMCPIDForCLIName(lsp-gopls) = %q, want %q", mcpID, "lsp-gopls")
	}
}

func TestResolveMCPIDForCLIName_EmptyInput(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	_, err := app.resolveMCPIDForCLIName("   ")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should reject empty input")
	}
	if !strings.Contains(err.Error(), "mcp name is required") {
		t.Fatalf("error = %v, want empty input validation", err)
	}
}

func TestResolveMCPIDForCLIName_NilRegistry(t *testing.T) {
	app := NewApp()

	_, err := app.resolveMCPIDForCLIName("gopls")
	if !errors.Is(err, errMCPRegistryNotInitialized) {
		t.Fatalf("resolveMCPIDForCLIName error = %v, want %v", err, errMCPRegistryNotInitialized)
	}
}

func TestResolveMCPIDForCLIName_NoDefinitions(t *testing.T) {
	app := newAppWithMCPManager(t)

	_, err := app.resolveMCPIDForCLIName("gopls")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should fail when no definitions are registered")
	}
	if !strings.Contains(err.Error(), "no mcp definitions are registered") {
		t.Fatalf("error = %v, want empty registry message", err)
	}
}

func TestResolveMCPIDForCLIName_GenericLauncherExcluded(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "memory", Name: "Memory", Command: "npx"},
	)

	_, err := app.resolveMCPIDForCLIName("npx")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should not resolve generic launcher aliases")
	}
	if !strings.Contains(err.Error(), "unknown mcp") {
		t.Fatalf("error = %v, want unknown mcp error", err)
	}
}

func TestResolveMCPIDForCLIName_DenoLauncherExcluded(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "deno-mcp", Name: "Deno MCP", Command: "deno"},
	)

	_, err := app.resolveMCPIDForCLIName("deno")
	if err == nil {
		t.Fatal("resolveMCPIDForCLIName should not resolve deno as a generic launcher alias")
	}
	if !strings.Contains(err.Error(), "unknown mcp") {
		t.Fatalf("error = %v, want unknown mcp error", err)
	}
}

func TestResolveMCPIDForCLIName_CaseInsensitiveInputs(t *testing.T) {
	app := newAppWithMCPManager(t,
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
			mcpID, err := app.resolveMCPIDForCLIName(tt.input)
			if err != nil {
				t.Fatalf("resolveMCPIDForCLIName(%q) error = %v", tt.input, err)
			}
			if mcpID != "lsp-gopls" {
				t.Fatalf("resolveMCPIDForCLIName(%q) = %q, want %q", tt.input, mcpID, "lsp-gopls")
			}
		})
	}
}

func TestResolveMCPStdio_UsesAliasAndReturnsPipe(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	t.Cleanup(func() {
		app.mcpManager.Close()
	})

	sessionName := "session-stdio-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")
	resolved, err := app.ResolveMCPStdio(sessionName, "gopls")
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
	detail, err := app.mcpManager.GetDetail(sessionName, "lsp-gopls")
	if err != nil {
		t.Fatalf("GetDetail error = %v", err)
	}
	if !detail.Enabled {
		t.Fatal("detail.Enabled should be true after ResolveMCPStdio")
	}
}

func TestResolveMCPStdio_StatusStartingFallsBackToDeterministicPipe(t *testing.T) {
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

	app := newAppWithMCPManagerConfig(t, mcp.ManagerConfig{
		EmitFn: func(string, any) {},
		ResolveWorkDir: func(string) (string, error) {
			close(workDirStarted)
			<-releaseWorkDir
			return t.TempDir(), nil
		},
	}, mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"})
	t.Cleanup(func() {
		release()
		app.mcpManager.Close()
	})

	sessionName := "session-starting"
	resolved, err := app.ResolveMCPStdio(sessionName, "gopls")
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

	detail, err := app.mcpManager.GetDetail(sessionName, "lsp-gopls")
	if err != nil {
		t.Fatalf("GetDetail error = %v", err)
	}
	if detail.Status != mcp.StatusStarting {
		t.Fatalf("detail.Status = %q, want %q", detail.Status, mcp.StatusStarting)
	}

	release()
}

func TestResolveMCPStdio_EmptyInputs(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)

	_, err := app.ResolveMCPStdio("   ", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should reject empty session name")
	}
	if !strings.Contains(err.Error(), "session name is required") {
		t.Fatalf("session validation error = %v", err)
	}

	_, err = app.ResolveMCPStdio("session-a", "   ")
	if err == nil {
		t.Fatal("ResolveMCPStdio should reject empty mcp name")
	}
	if !strings.Contains(err.Error(), "mcp name is required") {
		t.Fatalf("mcp validation error = %v", err)
	}
}

func TestResolveMCPStdio_NilManager(t *testing.T) {
	app := NewApp()

	_, err := app.ResolveMCPStdio("session-a", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should fail when manager is nil")
	}
	if !errors.Is(err, errMCPManagerNotInitialized) {
		t.Fatalf("ResolveMCPStdio error = %v, want %v", err, errMCPManagerNotInitialized)
	}
}

func TestResolveMCPStdio_StatusStopped(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls", DefaultEnabled: true},
	)
	t.Cleanup(func() {
		app.mcpManager.Close()
	})

	_, err := app.ResolveMCPStdio("session-stopped", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should fail when the MCP remains stopped")
	}
	if !strings.Contains(err.Error(), "was stopped before becoming ready") {
		t.Fatalf("error = %v, want stopped-before-ready message", err)
	}
}

func TestResolveMCPStdio_StatusError(t *testing.T) {
	stateChanged := make(chan struct{}, 4)
	app := newAppWithMCPManagerConfig(t, mcp.ManagerConfig{
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
		app.mcpManager.Close()
	})

	if err := app.mcpManager.SetEnabled("session-error", "lsp-gopls", true); err != nil {
		t.Fatalf("SetEnabled error = %v", err)
	}
	detail := waitForMCPDetailStatusEvent(t, app, stateChanged, "session-error", "lsp-gopls", mcp.StatusError)
	if !detail.Enabled {
		t.Fatal("detail.Enabled = false, want true after failed startup")
	}

	_, err := app.ResolveMCPStdio("session-error", "gopls")
	if err == nil {
		t.Fatal("ResolveMCPStdio should fail when the MCP is already in error state")
	}
	if !strings.Contains(err.Error(), "failed to start") {
		t.Fatalf("error = %v, want failed-to-start message", err)
	}
}

func TestRollbackResolvedMCP_DisablesOnlyNewlyEnabledMCP(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	t.Cleanup(func() {
		app.mcpManager.Close()
	})

	sessionName := "session-rollback-disable"
	if err := app.mcpManager.SetEnabled(sessionName, "lsp-gopls", true); err != nil {
		t.Fatalf("SetEnabled error = %v", err)
	}

	cause := errors.New("startup failed")
	err := rollbackResolvedMCP(app.mcpManager, sessionName, "lsp-gopls", true, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("rollbackResolvedMCP error = %v, want wrapped cause %v", err, cause)
	}

	detail, detailErr := app.mcpManager.GetDetail(sessionName, "lsp-gopls")
	if detailErr != nil {
		t.Fatalf("GetDetail error = %v", detailErr)
	}
	if detail.Enabled {
		t.Fatal("detail.Enabled = true, want false after rollback")
	}
}

func TestRollbackResolvedMCP_PreservesPreviouslyEnabledMCP(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	t.Cleanup(func() {
		app.mcpManager.Close()
	})

	sessionName := "session-rollback-preserve"
	if err := app.mcpManager.SetEnabled(sessionName, "lsp-gopls", true); err != nil {
		t.Fatalf("SetEnabled error = %v", err)
	}

	cause := errors.New("startup failed")
	err := rollbackResolvedMCP(app.mcpManager, sessionName, "lsp-gopls", false, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("rollbackResolvedMCP error = %v, want wrapped cause %v", err, cause)
	}

	detail, detailErr := app.mcpManager.GetDetail(sessionName, "lsp-gopls")
	if detailErr != nil {
		t.Fatalf("GetDetail error = %v", detailErr)
	}
	if !detail.Enabled {
		t.Fatal("detail.Enabled = false, want true when rollback is skipped")
	}
}

func TestRollbackResolvedMCP_JoinsRollbackFailure(t *testing.T) {
	app := newAppWithMCPManager(t,
		mcp.MCPDefinition{ID: "lsp-gopls", Name: "Go LSP", Command: "gopls"},
	)
	sessionName := "session-rollback-join"
	cause := errors.New("startup failed")

	app.mcpManager.Close()
	err := rollbackResolvedMCP(app.mcpManager, sessionName, "lsp-gopls", true, cause)
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

func TestLspExtensionMetaToDefinitions(t *testing.T) {
	metas := []lsppkg.ExtensionMeta{
		{Name: "gopls", Language: "Go", DefaultCommand: "gopls"},
		{Name: "rust", Language: "Rust", DefaultCommand: "rust-analyzer"},
	}
	defs := lspExtensionMetaToDefinitions(metas)
	if len(defs) != 2 {
		t.Fatalf("lspExtensionMetaToDefinitions() len = %d, want 2", len(defs))
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

func TestLspExtensionMetaToDefinitions_Empty(t *testing.T) {
	defs := lspExtensionMetaToDefinitions(nil)
	if defs != nil {
		t.Fatalf("lspExtensionMetaToDefinitions(nil) = %v, want nil", defs)
	}
}

func TestLspExtensionMetaToDefinitions_IntegrationWithAllExtensionMeta(t *testing.T) {
	metas := lsppkg.AllExtensionMeta()
	defs := lspExtensionMetaToDefinitions(metas)
	if len(defs) == 0 {
		t.Fatal("lspExtensionMetaToDefinitions(AllExtensionMeta()) returned empty list")
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
