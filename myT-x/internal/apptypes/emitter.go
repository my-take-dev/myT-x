package apptypes

import "context"

// RuntimeEventEmitter emits frontend events without depending on Wails runtime APIs.
// All layers (tmux command router, services, etc.) share this single interface.
type RuntimeEventEmitter interface {
	Emit(name string, payload any)
	EmitWithContext(ctx context.Context, name string, payload any)
}

// EventEmitterFunc adapts a plain func(name, payload) into RuntimeEventEmitter.
// EmitWithContext delegates to the same function, ignoring the context.
type EventEmitterFunc func(name string, payload any)

func (f EventEmitterFunc) Emit(name string, payload any) {
	f(name, payload)
}

// EmitWithContext delegates to the underlying function, intentionally ignoring
// the context. This is by design: EventEmitterFunc wraps simple event dispatch
// functions that do not support cancellation or deadline propagation.
// TODO: If context-aware cancellation (e.g. dropping events on ctx.Done())
// becomes necessary, introduce a ContextAwareEmitterFunc variant rather than
// adding context handling here, to preserve backward compatibility.
func (f EventEmitterFunc) EmitWithContext(_ context.Context, name string, payload any) {
	f(name, payload)
}

// NoopEmitter implements RuntimeEventEmitter as a no-op.
// Used as the default when a service's Emitter dependency is nil.
type NoopEmitter struct{}

func (NoopEmitter) Emit(string, any)                             {}
func (NoopEmitter) EmitWithContext(context.Context, string, any) {}
