package main

import (
	"context"
	"log/slog"

	"myT-x/internal/apptypes"
	"myT-x/internal/snapshot"
)

// appRuntimeEventEmitterAdapter adapts App runtime event helpers to apptypes.RuntimeEventEmitter.
type appRuntimeEventEmitterAdapter struct {
	app *App
}

func newAppRuntimeEventEmitterAdapter(app *App) apptypes.RuntimeEventEmitter {
	if app == nil {
		panic("newAppRuntimeEventEmitterAdapter: app must not be nil")
	}
	return appRuntimeEventEmitterAdapter{app: app}
}

func (e appRuntimeEventEmitterAdapter) Emit(name string, payload any) {
	e.app.emitRuntimeEvent(name, payload)
}

func (e appRuntimeEventEmitterAdapter) EmitWithContext(ctx context.Context, name string, payload any) {
	e.app.emitRuntimeEventWithContext(ctx, name, payload)
}

// emitRuntimeEvent emits via the app context and delegates to emitRuntimeEventWithContext.
func (a *App) emitRuntimeEvent(name string, payload any) {
	a.emitRuntimeEventWithContext(a.runtimeContext(), name, payload)
}

// emitRuntimeEventWithContext emits a runtime event only when ctx is non-nil.
// Prefer this helper for best-effort contexts that may not be initialized yet.
func (a *App) emitRuntimeEventWithContext(ctx context.Context, name string, payload any) {
	if ctx == nil {
		slog.Warn("[EVENT] runtime event dropped because app context is nil", "event", name)
		return
	}
	runtimeEventsEmitFn(ctx, name, payload)
}

// emitBackendEvent handles backend-originated runtime events.
// For tmux pane output, it delegates to the snapshot service.
// For other events, it emits the event and triggers snapshots per policy.
func (a *App) emitBackendEvent(name string, payload any) {
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[EVENT] backend event dropped because runtime context is nil", "event", name)
		return
	}
	if name == "app:activate-window" {
		a.bringWindowToFront()
		return
	}
	if name == "tmux:pane-output" {
		a.snapshotService.HandlePaneOutputEvent(payload)
		return
	}

	newAppRuntimeEventEmitterAdapter(a).EmitWithContext(ctx, name, payload)
	if shouldEmit, bypassDebounce := snapshot.SnapshotPolicyForEvent(name); shouldEmit {
		a.snapshotService.RequestSnapshot(bypassDebounce)
	}
}
