package main

import (
	"errors"
	"strings"

	"myT-x/internal/mcp"
	"myT-x/internal/mcpapi"
	"myT-x/internal/tmux"
)

// errSessionNotInitialized is returned when the session manager has not been
// initialized. This sentinel error enables callers to use errors.Is() for
// programmatic detection of uninitialized state.
var errSessionNotInitialized = errors.New("session manager is unavailable")

// errRouterNotInitialized is returned when the command router has not been
// initialized. This sentinel error enables callers to use errors.Is() for
// programmatic detection of uninitialized state, consistent with errSessionNotInitialized.
var errRouterNotInitialized = errors.New("router is unavailable")

func (a *App) requireSessions() (*tmux.SessionManager, error) {
	if a.sessions == nil {
		return nil, errSessionNotInitialized
	}
	return a.sessions, nil
}

func (a *App) requireRouter() (*tmux.CommandRouter, error) {
	if a.router == nil {
		return nil, errRouterNotInitialized
	}
	return a.router, nil
}

// requireSessionsWithPaneID trims, validates paneID, and returns the session manager.
// Consolidates the repeated TrimSpace + empty-check + requireSessions boilerplate
// used by pane-targeting public API methods.
//
// NOTE: paneID is accepted as a pointer so this function can normalize the caller's
// variable in place (TrimSpace). This avoids requiring every call site to reassign
// the trimmed value, which was a common source of bugs before this helper existed.
//
// Precondition: paneID must not be nil. All current call sites pass the address of
// a local variable, so this is guaranteed. A nil guard is included for defensive
// safety against future internal callers.
func (a *App) requireSessionsWithPaneID(paneID *string) (*tmux.SessionManager, error) {
	if paneID == nil {
		return nil, errors.New("paneID pointer must not be nil")
	}
	*paneID = strings.TrimSpace(*paneID)
	if *paneID == "" {
		return nil, errors.New("pane id is required")
	}
	return a.requireSessions()
}

// errMCPManagerNotInitialized is returned when the MCP manager has not been
// initialized. This sentinel error enables callers to use errors.Is() for
// programmatic detection of uninitialized state, consistent with errSessionNotInitialized.
var errMCPManagerNotInitialized = errors.New("mcp manager is unavailable")

// errMCPRegistryNotInitialized is returned when the MCP registry has not been
// initialized. This sentinel error enables callers to use errors.Is() for
// programmatic detection of uninitialized state in registry-only code paths.
var errMCPRegistryNotInitialized = errors.New("mcp registry is unavailable")

func (a *App) requireMCPManager() (*mcp.Manager, error) {
	if a.mcpManager == nil {
		return nil, errMCPManagerNotInitialized
	}
	return a.mcpManager, nil
}

func (a *App) requireMCPRegistry() (*mcp.Registry, error) {
	if a.mcpRegistry == nil {
		return nil, errMCPRegistryNotInitialized
	}
	return a.mcpRegistry, nil
}

// errMCPAPIServiceNotInitialized is returned when the MCP API service has not
// been initialized. Consistent with other require* guard sentinels.
var errMCPAPIServiceNotInitialized = errors.New("mcp api service is unavailable")

func (a *App) requireMCPAPIService() (*mcpapi.Service, error) {
	if a.mcpAPIService == nil {
		return nil, errMCPAPIServiceNotInitialized
	}
	return a.mcpAPIService, nil
}

func (a *App) requireSessionsAndRouter() (*tmux.SessionManager, *tmux.CommandRouter, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, nil, err
	}
	router, err := a.requireRouter()
	if err != nil {
		return nil, nil, err
	}
	return sessions, router, nil
}
