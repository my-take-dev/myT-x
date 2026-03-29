package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	runtimeWindowIsMinimisedFn    = runtime.WindowIsMinimised
	runtimeWindowHideFn           = runtime.WindowHide
	runtimeWindowShowFn           = runtime.WindowShow
	runtimeWindowUnminimiseFn     = runtime.WindowUnminimise
	runtimeWindowSetAlwaysOnTopFn = runtime.WindowSetAlwaysOnTop
	errRuntimeContextNil          = errors.New("runtime context is nil")
)

func (a *App) configureGlobalHotkey() {
	cfg := a.configState.Snapshot()
	// Early return: hotkeys backend not available (e.g. unsupported platform or test env).
	if a.hotkeys == nil {
		slog.Debug("[HOTKEY] hotkey backend unavailable, skipping registration")
		return
	}
	// Early return: quake-mode disabled in config — global hotkey is only for quake toggle.
	if !cfg.QuakeMode {
		slog.Debug("[HOTKEY] quake-mode disabled, skipping global hotkey registration")
		return
	}
	logCtx := a.runtimeContext()
	spec := strings.TrimSpace(cfg.GlobalHotkey)
	if spec == "" {
		slog.Debug("[HOTKEY] global hotkey is empty, skipping registration")
		return
	}

	if err := a.hotkeys.Start(spec, a.toggleQuakeWindow); err != nil {
		runtimeLogger.Warningf(logCtx, "global hotkey registration failed: %v", err)
		return
	}
	runtimeLogger.Infof(logCtx, "global hotkey registered: %s", a.hotkeys.ActiveBinding())
}

// bringWindowToFront shows and raises the application window.
// Used when a second instance signals the first to activate.
func (a *App) bringWindowToFront() error {
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[IPC] bringWindowToFront dropped because runtime context is nil")
		return errRuntimeContextNil
	}
	a.raiseWindow(ctx)
	a.setWindowVisible(true)
	return nil
}

func (a *App) raiseWindow(ctx context.Context) {
	runtimeWindowShowFn(ctx)
	runtimeWindowUnminimiseFn(ctx)
	runtimeWindowSetAlwaysOnTopFn(ctx, true)
	runtimeWindowSetAlwaysOnTopFn(ctx, false)
}

func (a *App) setWindowVisible(visible bool) {
	a.windowMu.Lock()
	a.windowVisible = visible
	a.windowMu.Unlock()
}

func (a *App) toggleQuakeWindow() {
	// CAS guard prevents double-toggle when a second hotkey fires
	// while OS window operations are in progress. Without this guard,
	// rapid hotkey presses could interleave Show/Hide calls, leaving
	// the window in an indeterminate visible/hidden state because the
	// OS window operations (Show, Hide, SetAlwaysOnTop) are not atomic.
	if !a.windowToggling.CompareAndSwap(false, true) {
		slog.Debug("[HOTKEY] toggle already in progress, skipping")
		return
	}
	defer a.windowToggling.Store(false)

	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[HOTKEY] toggleQuakeWindow dropped because runtime context is nil")
		return
	}

	// Read OS window state outside lock (#78: no Wails runtime API inside mutex).
	isMinimised := runtimeWindowIsMinimisedFn(ctx)

	// Determine action under lock.
	a.windowMu.Lock()
	currentlyVisible := a.windowVisible && !isMinimised
	a.windowMu.Unlock()

	// Perform OS window operations outside lock.
	if currentlyVisible {
		runtimeWindowHideFn(ctx)
	} else {
		a.raiseWindow(ctx)
	}

	a.setWindowVisible(!currentlyVisible)
}
