package taskscheduler

import (
	"context"

	"myT-x/internal/apptypes"
	"myT-x/internal/workerutil"
)

// Deps holds external dependencies injected at construction time.
// All function fields except Emitter and IsShuttingDown must be non-nil.
// NewService panics if any required function field is nil.
//
// Optional fields:
//   - Emitter: defaults to a no-op emitter if nil.
//   - IsShuttingDown: defaults to func() bool { return false } if nil.
type Deps struct {
	// Emitter sends runtime events to the frontend.
	// Optional: defaults to a no-op emitter if nil.
	Emitter apptypes.RuntimeEventEmitter

	// IsShuttingDown returns true when the application is tearing down.
	// Optional: defaults to func() bool { return false } if nil.
	IsShuttingDown func() bool

	// CheckPaneAlive returns nil if the pane exists, or an error describing
	// why it is unavailable.
	CheckPaneAlive func(paneID string) error

	// SendMessagePaste delivers a text message to the target pane using
	// bracketed-paste mode with Enter key.
	SendMessagePaste func(paneID, message string) error

	// ResolveOrchestratorDBPath returns the filesystem path to orchestrator.db.
	ResolveOrchestratorDBPath func() (string, error)

	// NewContext creates a cancellable context derived from the app runtime context.
	NewContext func() (context.Context, context.CancelFunc)

	// LaunchWorker starts a background goroutine with panic recovery
	// and WaitGroup tracking.
	LaunchWorker func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions)

	// BaseRecoveryOptions returns the default RecoveryOptions for background workers.
	BaseRecoveryOptions func() workerutil.RecoveryOptions

	// SendClearCommand sends a clear command (e.g. /new) to the target pane.
	// Uses literal SendKeys (non-paste) as the command is a short single-line input.
	SendClearCommand func(paneID, command string) error

	// GetSessionPaneIDs returns all pane IDs in the named session.
	GetSessionPaneIDs func(sessionName string) ([]string, error)

	// IsPaneQuiet returns true when the pane has had no recent output.
	IsPaneQuiet func(paneID string) bool

	// IsAgentTeamSession reports whether the session is running in agent-team mode.
	IsAgentTeamSession func(sessionName string) bool

	// SessionName is the session this service instance is bound to.
	SessionName string
}

// validateRequired panics if any required field is nil.
func (d *Deps) validateRequired() {
	if d.CheckPaneAlive == nil || d.SendMessagePaste == nil ||
		d.ResolveOrchestratorDBPath == nil || d.NewContext == nil ||
		d.LaunchWorker == nil || d.BaseRecoveryOptions == nil ||
		d.SendClearCommand == nil || d.GetSessionPaneIDs == nil ||
		d.IsPaneQuiet == nil || d.IsAgentTeamSession == nil {
		panic("taskscheduler.NewService: required function fields in Deps must be non-nil " +
			"(CheckPaneAlive, SendMessagePaste, ResolveOrchestratorDBPath, NewContext, " +
			"LaunchWorker, BaseRecoveryOptions, SendClearCommand, GetSessionPaneIDs, " +
			"IsPaneQuiet, IsAgentTeamSession)")
	}
}

// applyDefaults fills optional fields with sensible defaults.
func (d *Deps) applyDefaults() {
	if d.IsShuttingDown == nil {
		d.IsShuttingDown = func() bool { return false }
	}
	if d.Emitter == nil {
		d.Emitter = apptypes.NoopEmitter{}
	}
}
