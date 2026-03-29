package main

import (
	"fmt"
	"log/slog"

	"myT-x/internal/install"
)

var (
	cleanupLegacyShimInstallsFn = install.CleanupLegacyShimInstalls
	needsShimInstallFn          = install.NeedsShimInstall
	ensureShimInstalledFn       = install.EnsureShimInstalled
	resolveShimInstallDirFn     = install.ResolveInstallDir
	ensureProcessPathContainsFn = install.EnsureProcessPathContains
)

// ensureShimReady synchronizes the tmux shim on every startup and updates
// the current process PATH so child panes can find the shim binary.
// This prevents stale shim binaries when a new version is distributed.
func (a *App) ensureShimReady(workspace string) {
	// Remove legacy shim directories and stale PATH entries before checking
	// installation state. This ensures NeedsShimInstall sees a clean PATH.
	if err := cleanupLegacyShimInstallsFn(); err != nil {
		slog.Warn("[shim] legacy cleanup failed", "error", err)
	}

	needsInstallBefore, preCheckErr := needsShimInstallFn()
	if preCheckErr != nil {
		slog.Warn("[shim] detection failed", "error", preCheckErr)
	}

	result, installErr := ensureShimInstalledFn(workspace)
	if installErr != nil {
		slog.Warn("[shim] startup sync failed", "error", installErr)
		a.addPendingConfigLoadWarning(
			fmt.Sprintf("tmux shim installation failed at startup. Agent Team features may be unavailable. Error: %v", installErr),
		)
	} else {
		slog.Info("[shim] synchronized", "path", result.InstalledPath)
		// Preserve existing event behavior for first-time install scenarios.
		if eventCtx := a.runtimeContext(); needsInstallBefore && eventCtx != nil {
			runtimeEventsEmitFn(eventCtx, "tmux:shim-installed", result)
		}
	}

	// Ensure the shim directory is in the current process PATH so that
	// child processes (panes) inherit it and can find the tmux binary.
	shimDir, dirErr := resolveShimInstallDirFn()
	if dirErr == nil {
		if ensureProcessPathContainsFn(shimDir) {
			slog.Info("[shim] process PATH updated", "shimDir", shimDir)
		}
	} else {
		slog.Warn("[shim] resolveShimInstallDirFn failed", "error", dirErr)
	}

	// Final check: update shimAvailable based on current state.
	// preCheckErr / postCheckErr naming mirrors the before/after install phases.
	// When postCheckErr != nil the needsInstallAfter value defaults to false
	// (zero value), which intentionally causes SetShimAvailable(false) — the
	// conservative safe default.
	needsInstallAfter, postCheckErr := needsShimInstallFn()
	if postCheckErr != nil {
		slog.Warn("[shim] post-install check failed", "error", postCheckErr)
	}
	if a.router != nil {
		a.router.SetShimAvailable(!needsInstallAfter && postCheckErr == nil)
	}
}
